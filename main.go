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
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"os"
)

func main() { os.Exit(mainWithExitCode()) }
func mainWithExitCode() (exit_code int) {
	var err error
	var print_if_panic string
	var file *os.File
	exit_code = 0
	defer func() {
		if r := recover(); r != nil {
			if len(print_if_panic) != 0 {
				log.Print(print_if_panic)
			}
			exit_code = 2
		}
	}()

	flag.Parse()

	// check if flags are correct
	err = PostParsingCheckFlags()
	if err != nil {
		log.Print(flag.Arg(0), " untouched")
		log.Print("main, PostParsingCheckFlags err=\"", err, "\"")
		return 1
	}

	if collapse_test { // --collapse-test
		err = TestCollapse()
		if err != nil {
			fmt.Println("Collapse test : FAIL")
			return 1
		}
		fmt.Println("Collapse test : OK")
		return 0
	}

	// open source file
	file, err = os.OpenFile(flag.Arg(0), os.O_RDWR, 0644)
	if err != nil {
		log.Print(flag.Arg(0), " untouched")
		log.Print("main, os.OpenFile err=\"", err, "\"")
		return 1
	}
	defer file.Close()

	// main function
	print_if_panic = fmt.Sprint(flag.Arg(0), " may have been modified")
	file_total_byte_deallocated, _ := CopyWhileDeallocate(file, os.Stdout)

	if collapse { // --collapse

		print_if_panic = fmt.Sprint(flag.Arg(0), " dumped but collapse fail")

		// we can't collapse the whole file, so we make sure to keep at
		// least one byte
		_, err = CollapseFileStart(file, file_total_byte_deallocated - 1)
		if err != unix.EOPNOTSUPP {
			log.Print(flag.Arg(0), " dumped but collapse fail")
			log.Print("main, CollapseFileStart err=\"", err, "\"")
			return 1
		}

	} else if truncate { // --truncate

		// erase (collapse) the read bytes from file
		err = unix.Ftruncate(int(file.Fd()), 0)
		if err != nil {
			log.Print(flag.Arg(0), " dumped but truncate fail")
			log.Print("main, unix.Ftruncate err=\"", err, "\"")
			return 1
		}

	} else if remove { // --remove

		// before removing it, we close file
		err = file.Close()
		// file.Close() will be call by defer
		// so we "disable" it by making file nil
		file = nil
		if err != nil {
			log.Printf("%s dumped but close fail", flag.Arg(0))
			log.Print("main, file.Close err=\"", err, "\"")
			return 1
		}

		// remove file
		err = os.Remove(flag.Arg(0))
		if err != nil {
			log.Printf("%s dumped but remove fail", flag.Arg(0))
			log.Print("main, os.Remove err=\"", err, "\"")
			return 1
		}
	}
	return 0
}
