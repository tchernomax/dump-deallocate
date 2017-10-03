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
	"os"
	"strconv"
	"strings"
)

// boolean corresponding to flags
var collapse, collapseTest, truncate, remove bool
var collapseDefault, collapseTestDefault, truncateDefault, removeDefault bool = false, false, false, false

// sizeType is used for --buffer-size
type sizeType int64

var bufferSize sizeType = 32 * 1024 /* 32KiB */

// used by the "flag" package to handle --buffer-size parsing
func (sizeObj *sizeType) String() string {
	return fmt.Sprintf("%d", int64(*sizeObj))
}

/**
 * This function is used by the "flag" package to handle --buffer-size parsing.
 * It transforme …KiB, MiB, KB, etc. in int64.
 *
 * Can return: nil, strconv errors, errorNegativeOrZero or errorInt64Overflow
 */
func (sizeObj *sizeType) Set(sizeStr string) (err error) {
	var sizeInt int64

	if !strings.HasSuffix(sizeStr, "B") {
		// if the number in sizeStr is to big to fit in int64
		// ParseInt raise an error
		sizeInt, err = strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			// I return err…Err instead of err
			// because it make test easier
			return err.(*strconv.NumError).Err
		}
		if sizeInt <= 0 {
			return errorNegativeOrZero
		}
		// assign sizeInt to sizeObj (bufferSize value)
		*sizeObj = sizeType(sizeInt)
		return nil
	}

	sizeStrModified := sizeStr
	// remove "B" suffix
	sizeStrModified = sizeStrModified[:len(sizeStrModified)-1]

	base1024 := false /* base 1000 */
	if strings.HasSuffix(sizeStrModified, "i") {
		base1024 = true
		// remove "i" suffix
		sizeStrModified = sizeStrModified[:len(sizeStrModified)-1]
	}

	// K → power = 1
	// M → power = 2
	// G → power = 3
	// …
	power := 0
	sizeSuffixPower := [6]string{"K", "M", "G", "T", "P", "E"}
	for powerIndex, powerSuffix := range sizeSuffixPower {
		if strings.HasSuffix(sizeStrModified, powerSuffix) {
			power = powerIndex + 1
			// remove the suffix
			sizeStrModified = sizeStrModified[:len(sizeStrModified)-1]
			break
		}
	}

	// if the number in sizeStr is to big to fit in int64
	// ParseInt raise an error
	sizeInt, err = strconv.ParseInt(sizeStrModified, 10, 64)
	if err != nil {
		// I return err…Err instead of err
		// because it make test easier
		return err.(*strconv.NumError).Err
	}
	if sizeInt <= 0 {
		return errorNegativeOrZero
	}

	multiplicator := int64(1)
	if base1024 {
		// 1*(1024^power)
		multiplicator = (1 << (uint(power) * 10))
	} else /* base 1000 */ {
		// quick and durty way to do : 1*(1000^power)
		for i := 0; i < power; i++ {
			multiplicator = multiplicator * 1000
		}
	}

	// sizeInt = sizeInt * multiplicator
	// and check if we do integer overflow
	sizeIntBeforeMultiply := sizeInt
	sizeInt = sizeIntBeforeMultiply * multiplicator
	if sizeInt/multiplicator != sizeIntBeforeMultiply {
		return errorInt64Overflow
	}

	// assign sizeInt to sizeObj (bufferSize value)
	*sizeObj = sizeType(sizeInt)
	return nil
}

var errorNegativeOrZero = errors.New("negative or zero value")
var errorInt64Overflow = errors.New("value too big to fit in int64")

func init() {
	// bufferSize
	flag.Var(&bufferSize, "bufferSize", "")
	flag.Var(&bufferSize, "b", "")

	// collapse
	flag.BoolVar(&collapse, "collapse", collapseDefault, "")
	flag.BoolVar(&collapse, "c", collapseDefault, "")

	// collapseTest
	collapseTestDefault := false
	flag.BoolVar(&collapseTest, "collapse-test", collapseTestDefault, "")
	flag.BoolVar(&collapseTest, "C", collapseTestDefault, "")

	// truncate
	truncateDefault := false
	flag.BoolVar(&truncate, "truncate", truncateDefault, "")
	flag.BoolVar(&truncate, "t", truncateDefault, "")

	// remove
	removeDefault := false
	flag.BoolVar(&remove, "remove", removeDefault, "")
	flag.BoolVar(&remove, "r", removeDefault, "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [-b BYTES] [-c|-t|-r] FILE\n"+
				" Dump FILE on stdout and deallocate it at the same time.\n"+
				" More precisely:\n"+
				"   1. read BYTES bytes from FILE\n"+
				"   2. write thoses bytes on stdout\n"+
				"   3. deallocate BYTES bytes from FILE (fallocate punch-hole)\n"+
				"      and go back to 1.\n\n"+

				"Options:\n"+
				" -b, --buffer-size BYTES\n"+
				"        Memory buffer size in byte (default %dKiB).\n"+
				"        BYTES  may  be followed by the following multiplicative suffixes:\n"+
				"         - IEC unit:\n"+
				"           - KiB = 1024\n"+
				"           - MiB = 1024×1024\n"+
				"           …\n"+
				"           - EiB = 1024⁶\n"+
				"         - SI unit:\n"+
				"           - KB = 1000\n"+
				"           - MB = 1000×1000\n"+
				"           …\n"+
				"           - EB = 1000⁶\n\n"+

				" -c, --collapse\n"+
				"        At the end of the whole dump, remove/collapse (with fallocate collapse-range)\n"+
				"        the greatest number of filesystem blocks already dumped.\n"+
				"        On normal condition, at the end, FILE will size one filesystem block.\n\n"+

				"        Supported on ext4 from Linux 3.15.\n\n"+

				" -C, --collapse-test\n"+
				"        Test the collapse functionnality.\n"+
				"        Create a file named dump-deallocate-collapse-test-<random str>\n"+
				"        in the working directory and try to collapse it.\n"+
				"        Remove file after the test.\n\n"+

				" -t, --truncate\n"+
				"        Truncate FILE (to size 0) at the end of the whole dump\n"+
				"        It is not recommended since another process can write in FILE between\n"+
				"        the last read and the truncate call.\n"+
				"        On normal condition, at the end, FILE will size 0.\n\n"+

				" -r, --remove\n"+
				"        Remove FILE at the end of the whole dump\n"+
				"        It is not recommended since another process might be using FILE.\n\n"+

				"Example: dump-deallocate big.log | gzip > small.gz\n",
			os.Args[0], int64(bufferSize)/1024)
	}
}

/**
 * Verify some conditions on flags after the parsing.
 * Can return: nil, errorMissingFile, errorHaveFile or errorMutuallyExclusive
 */
func PostParsingCheckFlags() error {

	if flag.NArg() != 1 && !collapseTest {
		return errorMissingFile
	}

	if flag.NArg() != 0 && collapseTest {
		return errorHaveFile
	}

	if BoolToInt(collapse)+BoolToInt(collapseTest)+BoolToInt(truncate)+BoolToInt(remove) > 1 {
		return errorMutuallyExclusive
	}

	return nil
}

var errorMissingFile = errors.New("missing file parameter")
var errorHaveFile = errors.New("-C doesn't accept file parameter")
var errorMutuallyExclusive = errors.New("-c, -C, -t and -r are mutually exclusive")
