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
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"log"
	"os"
)

// Transform a boolean into integer
func BoolToInt(boolean bool) int {
	if boolean {
		return 1
	}
	return 0
}

/**
 * Get the filesystem block size where file is located
 *
 * Can Panic.
 */
func FilesystemBlockSize(file *os.File) int64 {

	var filesystemInfo unix.Statfs_t

	err := unix.Fstatfs(int(file.Fd()), &filesystemInfo)
	if err != nil {
		log.Panicf("FilesystemBlockSize, unix.Fstatfs err='%v'", err)
	}

	return filesystemInfo.Bsize
}

/**
 * Copy file to output while deallocating file.
 * Use a memory buffer of bufferSize.
 * Return the number of bytes deallocated and written, which should be equal.
 *
 * Can Panic.
 */
func CopyWhileDeallocate(file *os.File, output io.Writer) (fileTotalByteDeallocated int64, outputTotalByteWritten int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("fileTotalByteDeallocated: ", fileTotalByteDeallocated)
			log.Print("outputTotalByteWritten: ", outputTotalByteWritten)
			panic(r)
		}
	}()

	buffer := make([]byte, bufferSize)

	// main read→write loop
	for {

		nbByteRead, readError := file.Read(buffer)

		if nbByteRead > 0 {

			// write on output the bytes we just read in file
			nbByteWritten, err := output.Write(buffer[0:nbByteRead])
			outputTotalByteWritten += int64(nbByteWritten)

			if err != nil {
				log.Panicf("CopyWhileDeallocate, os.Stdout.Write err='%v'", err)
			}
			// fail to write as much byte as we read
			if nbByteRead != nbByteWritten {
				log.Panic("CopyWhileDeallocate, os.Stdout.Write: ", io.ErrShortWrite)
			}

			// deallocate the read bytes from file
			err = unix.Fallocate(int(file.Fd()),
				unix.FALLOC_FL_PUNCH_HOLE | unix.FALLOC_FL_KEEP_SIZE,
				fileTotalByteDeallocated,
				int64(nbByteRead))
			if err != nil {
				log.Panicf("CopyWhileDeallocate, unix.Fallocate punch-hole err='%v'", err)
			}

			/* man 2 fallocate:
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

			fileTotalByteDeallocated += int64(nbByteRead)
		}

		if readError == io.EOF {
			// the whole file has been read (and deallocated)
			// we stop here
			break
		}

		if readError != nil {
			log.Panic("CopyWhileDeallocate, file.Read: ", readError)
		}
	}

	return fileTotalByteDeallocated, outputTotalByteWritten
}

/**
 * Collapse (man 2 fallocate) file of the maximum number of byte possible less than bytesToDeallocate.
 * For exemple if file is 2 filesystem block (fsb), and you try to deallocate more bytes, the function will
 * deallocate 1 filesystem block (fallocate can't collapse the whole file).
 *
 * Can return errors: nil, errorZero, errorLessThanOneFsb and unix.EOPNOTSUPP.
 *
 * Can Panic.
 */
func CollapseFileStart(file *os.File, bytesToDeallocate int64) (byteActualyDeallocated int64, err error) {

	// fsb : file system block

	if bytesToDeallocate <= 0 {
		return 0, errorZero
	}

	// for COLLAPSE_RANGE, offset and len have to
	// be multiple of the filesystem block size
	// so we get filesystem informations (including block size)
	fsBlockSize := FilesystemBlockSize(file)

	// get number of fsb in the file
	var fileInfo unix.Stat_t
	err = unix.Fstat(int(file.Fd()), &fileInfo)
	if err != nil {
		log.Panicf("FilesystemBlockSize, unix.Fstat err='%v'", err)
	}

	fileSizeInFsb := fileInfo.Blocks / (fsBlockSize / 512)

	if fileSizeInFsb == 1 {
		return 0, errorLessThanOneFsb
	}

	// we make sure collapseLen is a multiple of the filesystem block size
	collapseLen := bytesToDeallocate - (bytesToDeallocate % fsBlockSize)
	collapseLenInFsb := collapseLen / fsBlockSize

	// we can't deallocate the whole file
	if collapseLenInFsb >= fileSizeInFsb {
		collapseLen -= fsBlockSize
		collapseLenInFsb -= 1
	}

	if collapseLenInFsb < 1 {
		return 0, errorZero
	}

	// collapse (erase) the greatest number of filesystem blocks already dumped/read
	err = unix.Fallocate(int(file.Fd()),
		unix.FALLOC_FL_COLLAPSE_RANGE,
		0,
		collapseLen)

	if err != nil && err != unix.EOPNOTSUPP {
		log.Panicf("CollapseFileStart, unix.Fallocate err='%v'", err)
	}

	return collapseLen, err
}

var errorZero = errors.New("try to deallocate 0 or less bytes")
var errorLessThanOneFsb = errors.New("can't collapse the file to less than one file system block")

/**
 * Test the collapse feature of fallocate.
 * The function create a temporary file on the working directory and try to collapse it.
 *
 * Can return nil or unix.EOPNOTSUPP.
 *
 * Can Panic.
 */
func TestCollapse() (err error) {

	// create the test file
	file, err := ioutil.TempFile(".", "dump-deallocate-collapse-test-")
	if err != nil {
		log.Panicf("TestCollapse, ioutil.TempFile err='%v'", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	// resize it to : 2 filesystem block size (as COLLAPSE_RANGE
	// len have to be multiple of the filesystem block size)
	fsBlockSize := FilesystemBlockSize(file)
	err = unix.Fallocate(int(file.Fd()),
		0, /* Default: allocate disk space*/
		0,
		2*fsBlockSize)
	if err != nil {
		log.Panicf("TestCollapse, unix.Fallocate err='%v'", err)
	}

	// try to collapse it's first filesystem block
	err = unix.Fallocate(int(file.Fd()),
		unix.FALLOC_FL_COLLAPSE_RANGE,
		0,
		fsBlockSize)

	if err != nil && err != unix.EOPNOTSUPP {
		log.Panicf("TestCollapse, unix.Fallocate err='%v'", err)
	}

	return err
}
