package main

import (
	"errors"
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

var err error
var src_total_byte_deallocated, dst_total_byte_written int64

/*
 * begin flags handling
 */
var collapse_range, truncate, remove bool

type size_type int64
var buffer_size size_type = 32*1024 /* 32KiB */
// used by "flag" to handle buffer_size
func (size_obj *size_type) String() string {
	return fmt.Sprintf("%d", int64(*size_obj))
}
/* used by "flag" to handle buffer_size
 * transforme …KiB, MiB, KB, etc. in int64 */
func (size_obj *size_type) Set(size_str string) error {
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
	size_suffix_power := [6]string{"K", "M", "G", "T", "P", "Z"}
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

	// collapse_range
	collapse_range_default := false
	flag.BoolVar(&collapse_range, "collapse-range", collapse_range_default, "")
	flag.BoolVar(&collapse_range, "c",              collapse_range_default, "")

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
		            "Usage: %s [options] file\n" +
		            " Dump 'file' on stdout and deallocate it at the same time.\n" +
		            " More precisely:\n" +
		            "   1. read 'buffer-size' bytes from 'file'\n" +
		            "   2. write thoses bytes on stdout\n" +
		            "   3. deallocate 'buffer-size' bytes from 'file' (fallocate punch-hole)\n" +
		            "      and go back to 1.\n\n" +

		            "Options:\n" +
		            " -b, --buffer-size int\n" +
		            "        memory buffer size in byte (default %dKiB)\n\n" +

		            " -c, --collapse-range\n" +
		            "        At the end of the whole dump, remove/collapse (with fallocate collapse-range)\n" +
		            "        the greatest number of filesystem blocks already dumped.\n" +
		            "        On normal condition, at the end, 'file' will size one filesystem block.\n\n" +

		            "        Supported on ext4 from Linux 3.15.\n\n" +

		            " -t, --truncate\n" +
		            "        Truncate the file (to size 0) at the end of the whole dump\n" +
		            "        It is not recommended since another process can write in the file between\n" +
		            "        the last read and the truncate call.\n" +
		            "        On normal condition, at the end, 'file' will size 0.\n\n" +

		            " -r, --remove\n" +
		            "        Remove the file at the end of the whole dump\n" +
		            "        It is not recommended since another process might be using the file.\n",
		            os.Args[0], int64(buffer_size) / 1024)
	}
}
/*
 * flags handling end
 */

func ExitIfError(exit_str string, err error) {
	if err == nil {
		return
	}

	// in case of non recoverable error, print some informations to the user
	log.Print("src_total_byte_deallocated: ", src_total_byte_deallocated)
	log.Print("dst_total_byte_written: ",     dst_total_byte_written)
	log.Fatal(exit_str, err)
}

func main() {
	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("Missing argument")
	}

	// open source file
	src_file, err := os.OpenFile(flag.Arg(0), os.O_RDWR, 0644)
	ExitIfError("os.OpenFile: ", err)
	defer src_file.Close()

	buffer := make([]byte, buffer_size)

	// main read→write loop
	for {

		nb_byte_read, read_error := src_file.Read(buffer)

		if nb_byte_read > 0 {

			// write the bytes we just read on stdout
			nb_byte_written, err := os.Stdout.Write(buffer[0:nb_byte_read])
			ExitIfError("os.Stdout.Write: ", err)

			dst_total_byte_written += int64(nb_byte_written)

			// fail to write as much byte as we read
			if nb_byte_read != nb_byte_written {
				ExitIfError("os.Stdout.Write: ", io.ErrShortWrite)
			}

			// erase (deallocate space) the read bytes from src_file
			err = unix.Fallocate(int(src_file.Fd()),
			                     0x02 /*FALLOC_FL_PUNCH_HOLE*/ | 0x01 /*FALLOC_FL_KEEP_SIZE*/,
			                     src_total_byte_deallocated,
			                     int64(nb_byte_read))
			ExitIfError("unix.Fallocate punch-hole: ", err)

			/* notes on Fallocate:
			*  FALLOC_FL_… : https://github.com/golang/go/issues/10599
			*
			*  man 2 fallocate:
			*  The FALLOC_FL_PUNCH_HOLE flag must be ORed with FALLOC_FL_KEEP_SIZE in mode
			*
			*  I can't use FALLOC_FL_COLLAPSE_RANGE (I tried) because
			*  the src_file read-seek-pointer isn't modified by fallocate :
			*    src_file : ' ← x already read bytes → read-seek-pointer ← unread bytes → '
			*    fallocate COLLAPSE_RANGE (x bytes are remove from the start of the file)
			*    src_file should be : ' read-seek-pointer (file start) ← unread bytes → '
			*    src_file is        : ' ← x unread bytes → read-seek-pointer ← unread bytes → '
			*  Also we can't fix the issue by moving the seek pointer because the src_file can be open
			*  by other process. On some conditions if this other process write on the file,
			*  the x bytes removed by fallocate are added back by the kernel (as zeros, sparse).
			*/

			src_total_byte_deallocated += int64(nb_byte_read)
		}

		if read_error == io.EOF {
			// the whole src_file has been read (and erased)
			// we stop here
			break
		}

		ExitIfError("src_file.Read: ", read_error)
	}

	if collapse_range {

		// for collapse_range, offset and len have to
		// be multiple of the filesystem block size
		// so we get filesystem informations (including block size)
		var src_filesystem_info unix.Statfs_t
		err = unix.Fstatfs(int(src_file.Fd()), &src_filesystem_info)
		ExitIfError("unix.Fstatfs: ", err)

		// we can't collapse the whole file, so we make sure to keep at
		// least one byte
		var collapse_len int64 = src_total_byte_deallocated - 1
		// we make sure collapse_len is a multiple of the filesystem block size
		collapse_len -= collapse_len % src_filesystem_info.Bsize
		if collapse_len < 0 {
			// the file already size one filesystem block
			return
		}
		// erase (collapse) the greatest number of filesystem blocks already dumped/read
		err = unix.Fallocate(int(src_file.Fd()),
		                     0x08 /*FALLOC_FL_COLLAPSE_RANGE*/,
		                     0,
		                     collapse_len)
		ExitIfError("unix.Fallocate collapse: ", err)

	} else if truncate {

		// erase (collapse) the read bytes from src_file
		err = unix.Ftruncate(int(src_file.Fd()), 0)
		ExitIfError("unix.Ftruncate: ", err)

	} else if remove {

		// before removing it, we close src_file
		err = src_file.Close()
		// src_file.Close() will be call by defer
		// so we "disable" it by making src_file nil
		src_file = nil
		ExitIfError("src_file.Close: ", err)

		// remove src_file
		err = os.Remove(flag.Arg(0))
		ExitIfError("os.Remove: ", err)
	}
}
