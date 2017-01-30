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

func main() {
	var err error
	var print_if_panic string
	var file *os.File
	defer func() {
		if r := recover(); r != nil {
			if len(print_if_panic) != 0 {
				log.Print(print_if_panic)
			}
			os.Exit(1)
		}
	}()

	flag.Parse()

	// check if flags are correct
	PostParsingCheckFlags()

	if collapse_test { // --collapse-test
		err = TestCollapse()
		if err != nil {
			fmt.Println("Collapse test : FAIL")
			os.Exit(1)
		}
		fmt.Println("Collapse test : OK")
		return
	}

	// open source file
	file, err = os.OpenFile(flag.Arg(0), os.O_RDWR, 0644)
	if err != nil {
		log.Print(flag.Arg(0), " untouched")
		log.Fatal("main, os.OpenFile err=\"", err, "\"")
	}
	defer file.Close()

	// main function
	print_if_panic = fmt.Sprint(flag.Arg(0), " may have been modified")
	file_total_byte_deallocated, _ := CopyWhileDeallocate(file, os.Stdout)

	if collapse { // --collapse

		print_if_panic = fmt.Sprint(flag.Arg(0), " dumped but collapse fail")

		CollapseFileStart(file, file_total_byte_deallocated)

	} else if truncate { // --truncate

		// erase (collapse) the read bytes from file
		err = unix.Ftruncate(int(file.Fd()), 0)
		if err != nil {
			log.Print(flag.Arg(0), " dumped but truncate fail")
			log.Fatal("main, unix.Ftruncate err=\"", err, "\"")
		}

	} else if remove { // --remove

		// before removing it, we close file
		err = file.Close()
		// file.Close() will be call by defer
		// so we "disable" it by making file nil
		file = nil
		if err != nil {
			log.Printf("%s dumped but close fail", flag.Arg(0))
			log.Fatal("main, file.Close err=\"", err, "\"")
		}

		// remove file
		err = os.Remove(flag.Arg(0))
		if err != nil {
			log.Printf("%s dumped but remove fail", flag.Arg(0))
			log.Fatal("main, os.Remove err=\"", err, "\"")
		}
	}
}
