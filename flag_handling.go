/**
 * dump-deallocate
 *
 * Copyright (C) 2017 Maxime de Roucy
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software Foundation,
 * Inc., 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301  USA
 */

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var collapse, truncate, remove bool

type size_type int64
var buffer_size size_type = 32*1024 /* 32KiB */
// used by "flag" to handle buffer_size
func (size_obj *size_type) String() string {
	return fmt.Sprintf("%d", int64(*size_obj))
}
/* used by "flag" to handle buffer_size
 * transforme …KiB, MiB, KB, etc. in int64 */
func (size_obj *size_type) Set(size_str string) (err error) {
	var size_int int64

	if ! strings.HasSuffix(size_str, "B") {
		// if the number in size_str is to big to fit in int64
		// ParseInt raise an error
		size_int, err = strconv.ParseInt(size_str, 10, 64)
		if err != nil {
			return err
		}
		if size_int <= 0 {
			return errors.New("negative value")
		}
		// assign size_int to size_obj (buffer_size value)
		*size_obj = size_type(size_int)
		return nil
	}

	size_str_modified := size_str
	// remove "B" suffix
	size_str_modified = size_str_modified[:len(size_str_modified)-1]

	base_1024 := false /* base 1000 */
	if strings.HasSuffix(size_str_modified, "i") {
		base_1024 = true
		// remove "i" suffix
		size_str_modified = size_str_modified[:len(size_str_modified)-1]
	}

	// K → power = 1
	// M → power = 2
	// G → power = 3
	// …
	power := 0
	size_suffix_power := [6]string{"K", "M", "G", "T", "P", "E"}
	for power_index,power_suffix := range size_suffix_power {
		if strings.HasSuffix(size_str_modified, power_suffix) {
			power = power_index + 1
			// remove the suffix
			size_str_modified = size_str_modified[:len(size_str_modified)-1]
			break
		}
	}

	// if the number in size_str is to big to fit in int64
	// ParseInt raise an error
	size_int, err = strconv.ParseInt(size_str_modified, 10, 64)
	if err != nil {
		return err
	}
	if size_int <= 0 {
		return errors.New("negative value")
	}

	multiplicator := int64(1)
	if base_1024 {
		// 1*(1024^power)
		multiplicator = ( 1 << (uint(power) * 10) )
	} else /* base 1000 */ {
		// quick and durty way to do : 1*(1000^power)
		for i := 0; i < power; i++ {
			multiplicator = multiplicator * 1000
		}
	}

	// size_int = size_int * multiplicator
	// and check if we do integer overflow
	size_int_before_multiply := size_int
	size_int = size_int_before_multiply * multiplicator
	if size_int / multiplicator != size_int_before_multiply {
		return errors.New("value too big to fit in int64")
	}

	// assign size_int to size_obj (buffer_size value)
	*size_obj = size_type(size_int)
	return nil
}

func init() {
	// buffer_size
	flag.Var(&buffer_size, "buffer_size", "")
	flag.Var(&buffer_size, "b", "")

	// collapse
	collapse_default := false
	flag.BoolVar(&collapse, "collapse", collapse_default, "")
	flag.BoolVar(&collapse, "c",              collapse_default, "")

	// truncate
	truncate_default := false
	flag.BoolVar(&truncate, "truncate", truncate_default, "")
	flag.BoolVar(&truncate, "t",        truncate_default, "")

	// remove
	remove_default := false
	flag.BoolVar(&remove, "remove", remove_default, "")
	flag.BoolVar(&remove, "r",      remove_default, "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
		            "Usage: %s [-b BYTES] [-c|-t|-r] FILE\n" +
		            " Dump FILE on stdout and deallocate it at the same time.\n" +
		            " More precisely:\n" +
		            "   1. read BYTES bytes from FILE\n" +
		            "   2. write thoses bytes on stdout\n" +
		            "   3. deallocate BYTES bytes from FILE (fallocate punch-hole)\n" +
		            "      and go back to 1.\n\n" +

		            "Options:\n" +
		            " -b, --buffer-size BYTES\n" +
		            "        Memory buffer size in byte (default %dKiB).\n" +
			    "        BYTES  may  be followed by the following multiplicative suffixes:\n" +
			    "         - IEC unit:\n" +
			    "           - KiB = 1024\n" +
			    "           - MiB = 1024×1024\n" +
			    "           …\n" +
			    "           - EiB = 1024⁶\n" +
			    "         - SI unit:\n" +
			    "           - KB = 1000\n" +
			    "           - MB = 1000×1000\n" +
			    "           …\n" +
			    "           - EB = 1000⁶\n\n" +

		            " -c, --collapse\n" +
		            "        At the end of the whole dump, remove/collapse (with fallocate collapse-range)\n" +
		            "        the greatest number of filesystem blocks already dumped.\n" +
		            "        On normal condition, at the end, FILE will size one filesystem block.\n\n" +

		            "        Supported on ext4 from Linux 3.15.\n\n" +

		            " -t, --truncate\n" +
		            "        Truncate FILE (to size 0) at the end of the whole dump\n" +
		            "        It is not recommended since another process can write in FILE between\n" +
		            "        the last read and the truncate call.\n" +
		            "        On normal condition, at the end, FILE will size 0.\n\n" +

		            " -r, --remove\n" +
		            "        Remove FILE at the end of the whole dump\n" +
		            "        It is not recommended since another process might be using FILE.\n",
		            os.Args[0], int64(buffer_size) / 1024)
	}
}

func PostParsingCheckFlags() {

	if flag.NArg() != 1 {
		log.Panic("PostParsingCheckFlags: missing argument")
	}

	if BoolToInt(collapse) + BoolToInt(truncate) + BoolToInt(remove) > 1 {
		log.Panic("PostParsingCheckFlags: -c, -t and -r are mutually exclusive")
	}
}
