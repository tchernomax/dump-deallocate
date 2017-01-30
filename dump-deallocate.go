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
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"log"
	"os"
)

// TODO document
func BoolToInt(boolean bool) int {
	if boolean {
		return 1
	}
	return 0
}

/**
 * Can Panic.
 */
func GetFilesystemBlockSize(file *os.File) int64 {

	var filesystem_info unix.Statfs_t
	err := unix.Fstatfs(int(file.Fd()), &filesystem_info)

	if err != nil {
		log.Panic("GetFilesystemBlockSize, unix.Fstatfs err=\"", err, "\"")
	}

	return filesystem_info.Bsize
}

/**
 * Can Panic.
 */
func CopyWhileDeallocate(file *os.File, output io.Writer) (file_total_byte_deallocated int64, stdout_total_byte_written int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("file_total_byte_deallocated: ", file_total_byte_deallocated)
			log.Print("stdout_total_byte_written: ", stdout_total_byte_written)
			panic(r)
		}
	}()

	buffer := make([]byte, buffer_size)

	// main read→write loop
	for {

		nb_byte_read, read_error := file.Read(buffer)

		if nb_byte_read > 0 {

			// write the bytes we just read on stdout
			nb_byte_written, err := output.Write(buffer[0:nb_byte_read])
			stdout_total_byte_written += int64(nb_byte_written)

			if err != nil {
				log.Panic("CopyWhileDeallocate, os.Stdout.Write err=\"", err, "\"")
			}
			// fail to write as much byte as we read
			if nb_byte_read != nb_byte_written {
				log.Panic("CopyWhileDeallocate, os.Stdout.Write: ", io.ErrShortWrite)
			}

			// deallocate the read bytes from file
			err = unix.Fallocate(int(file.Fd()),
			                     0x02 /*FALLOC_FL_PUNCH_HOLE*/ | 0x01 /*FALLOC_FL_KEEP_SIZE*/,
			                     file_total_byte_deallocated,
			                     int64(nb_byte_read))
			if err != nil {
				log.Panic("CopyWhileDeallocate, unix.Fallocate punch-hole err=\"", err, "\"")
			}

			/* notes on Fallocate:
			*  FALLOC_FL_… : https://github.com/golang/go/issues/10599
			*
			*  man 2 fallocate:
			*  The FALLOC_FL_PUNCH_HOLE flag must be ORed with FALLOC_FL_KEEP_SIZE in mode
			*
			*  I can't use FALLOC_FL_COLLAPSE_RANGE (I tried) because
			*  the file read-seek-pointer isn't modified by fallocate :
			*    file : ' ← x already read bytes → read-seek-pointer ← unread bytes → '
			*    fallocate COLLAPSE_RANGE (x bytes are remove from the start of the file)
			*    file should be : ' read-seek-pointer (file start) ← unread bytes → '
			*    file is        : ' ← x unread bytes → read-seek-pointer ← unread bytes → '
			*  Also we can't fix the issue by moving the seek pointer because the file can be open
			*  by other process. On some conditions if this other process write on the file,
			*  the x bytes removed by fallocate are added back by the kernel (as zeros, sparse).
			*/

			file_total_byte_deallocated += int64(nb_byte_read)
		}

		if read_error == io.EOF {
			// the whole file has been read (and deallocated)
			// we stop here
			break
		}

		if read_error != nil {
			log.Panic("CopyWhileDeallocate, file.Read: ", read_error)
		}
	}

	return file_total_byte_deallocated, stdout_total_byte_written
}

/**
 * Can Panic.
 */
func CollapseFileStart(file *os.File, bytes_to_deallocate int64) (byte_actualy_deallocated int64) {

	// for COLLAPSE_RANGE, offset and len have to
	// be multiple of the filesystem block size
	// so we get filesystem informations (including block size)
	fs_block_size := GetFilesystemBlockSize(file)

	// we can't collapse the whole file, so we make sure to keep at
	// least one byte
	var collapse_len int64 = bytes_to_deallocate - 1

	// we make sure collapse_len is a multiple of the filesystem block size
	collapse_len -= collapse_len % fs_block_size
	if collapse_len <= 0 {
		// the file already size one filesystem block
		return 0
	}

	// collapse (erase) the greatest number of filesystem blocks already dumped/read
	err := unix.Fallocate(int(file.Fd()),
	                     0x08 /*FALLOC_FL_COLLAPSE_RANGE*/,
	                     0,
	                     collapse_len)
	if err != nil {
		log.Panic("CollapseFileStart, unix.Fallocate err=\"", err, "\"")
	}

	return collapse_len
}

/**
 * Can Panic.
 */
func TestCollapse() (err error) {

	// create the test file
	file, err := ioutil.TempFile(".", "dump-deallocate-collapse-test-")
	if err != nil {
		log.Panic("TestCollapse, ioutil.TempFile err=\"", err, "\"")
	}
	defer os.Remove(file.Name())
	defer file.Close()

	// resize it to : 2 filesystem block size (as COLLAPSE_RANGE
	// len have to be multiple of the filesystem block size)
	fs_block_size := GetFilesystemBlockSize(file)
	err = unix.Fallocate(int(file.Fd()),
	                     0 /* Default: allocate disk space*/,
	                     0,
	                     2 * fs_block_size)

	if err != nil {
		log.Panic("TestCollapse, unix.Fallocate err=\"", err, "\"")
	}

	// try to collapse it's first filesystem block
	return unix.Fallocate(int(file.Fd()),
	                     0x08 /*FALLOC_FL_COLLAPSE_RANGE*/,
	                     0,
	                     fs_block_size)
}
