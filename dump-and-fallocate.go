package main

import (
    "os"
    "log"
    "io"
    "flag"
    "fmt"
    "golang.org/x/sys/unix"
)

var exit_error error = nil
var src_total_byte_deleted, dst_total_byte_written int64 = 0, 0

func ExitIfError(exit_error error, exit_str string) {
	if exit_error == nil {
		return
	}

	// in case of non recoverable error, print some informations
	// to the user
	log.Print("src_total_byte_deleted: ", src_total_byte_deleted)
	log.Print("dst_total_byte_written: ", dst_total_byte_written)
	log.Fatal(exit_str, exit_error)
}

func main() {
	// define cmdline arguments
	buffer_size    := flag.Int64("b", 32*1024, "buffer size in byte")
	collapse_range := flag.Bool("c", false, "collapse-range at the end of the process (supported on ext4 from Linux 3.15)")
	truncate       := flag.Bool("t", false, "truncate at the end of the process (unsafe)")

	flag.Usage = func() {
	        fmt.Fprintf(os.Stderr,  "Usage: %s [options] file\n", os.Args[0])
	        fmt.Fprintln(os.Stderr, " * 'file' will be dumped on stdout and fallocated (punch-hole) during the process.\n" +
		                        " * It's possible to remove the holes (collapse-range or truncate) at the end of the dump.\n" +
		                        "   - collapse-range: collapse the greatest number of filesystem blocks already dumped.\n" +
					"     On normal condition, at the end 'file' will size one filesystem block.\n" +
		                        "   - truncate: truncate the whole file, it is not recommended since another process\n" +
		                        "     can write in the file between the last read and the truncate call.\n" +
		                        "     On normal condition, at the end 'file' will size 0.\n" +
		                        "Options:")
	        flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("Missing argument")
	}

	// open source file
	src_file, exit_error := os.OpenFile(flag.Arg(0), os.O_RDWR, 0644)
	ExitIfError(exit_error, "os.OpenFile: ")

	buffer := make([]byte, *buffer_size)

	// main read→write loop
	for {

		nb_byte_read, read_error := src_file.Read(buffer)

		if nb_byte_read > 0 {

			// write the bytes we just read on stdout
			nb_byte_written, exit_error := os.Stdout.Write(buffer[0:nb_byte_read])
			ExitIfError(exit_error, "os.Stdout.Write: ")

			dst_total_byte_written += int64(nb_byte_written)

			// fail to write as much byte as we read
			if nb_byte_read != nb_byte_written {
				ExitIfError(io.ErrShortWrite, "os.Stdout.Write: ")
			}

			// erase (deallocate space) the read bytes from src_file
			exit_error = unix.Fallocate(int(src_file.Fd()),
			                            0x02 /*FALLOC_FL_PUNCH_HOLE*/ | 0x01 /*FALLOC_FL_KEEP_SIZE*/,
			                            src_total_byte_deleted,
			                            int64(nb_byte_read))
			ExitIfError(exit_error, "unix.Fallocate punch-hole: ")

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

			src_total_byte_deleted += int64(nb_byte_read)
		}

		if read_error == io.EOF {
			// the whole src_file has been read (and erased)
			// we stop here
			break
		}
		ExitIfError(read_error, "src_file.Read: ")
	}

	if *collapse_range {
		// for collapse_range, offset and len have to
		// be multiple of the filesystem block size
		// so we get filesystem informations (including block size)
		var src_filesystem_info unix.Statfs_t
		exit_error = unix.Fstatfs(int(src_file.Fd()), &src_filesystem_info)
		ExitIfError(exit_error, "unix.Fstatfs: ")

		// we can't collapse the whole file, so we make sure to keep at
		// leaste one byte
		var fallocate_len int64 = src_total_byte_deleted - 1
		// we make sure fallocate_len is a multiple of the filesystem block size
		fallocate_len -= fallocate_len % src_filesystem_info.Bsize
		if fallocate_len < 0 {
			return
		}
		// erase (collapse) the greatest number of filesystem blocks already dumped/read
		exit_error = unix.Fallocate(int(src_file.Fd()),
		                            0x08 /*FALLOC_FL_COLLAPSE_RANGE*/,
		                            0,
					    fallocate_len)
		ExitIfError(exit_error, "unix.Fallocate collapse: ")
	} else if *truncate {
		// erase (collapse) the read bytes from src_file
		exit_error = unix.Ftruncate(int(src_file.Fd()), 0)
		ExitIfError(exit_error, "unix.Ftruncate: ")
	}
}
