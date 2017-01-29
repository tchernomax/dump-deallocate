# dump-deallocate

## Usage

	dump-deallocate [-b BYTES] [-c|-t|-r] FILE

Dump FILE on stdout and deallocate it at the same time.
More precisely:

1. read BYTES bytes from FILE
2. write thoses bytes on stdout
3. deallocate BYTES bytes from FILE (fallocate punch-hole) and go back to 1.

Options:

-b, --buffer-size BYTES
: Memory buffer size in byte (default 32KiB).
	BYTES  may  be followed by the following multiplicative suffixes:

	* IEC unit:
		* KiB = 1024
		* MiB = 1024×1024
		…
		* EiB = 1024⁶
	* SI unit:
		* KB = 1000
		* MB = 1000×1000
		…
		* EB = 1000⁶

-c, --collapse
: At the end of the whole dump, remove/collapse (with fallocate collapse-range) the greatest number of filesystem blocks already dumped.
	On normal condition, at the end, FILE will size one filesystem block.  
	Supported on ext4 from Linux 3.15.

-C, --collapse-test
: Test the collapse functionnality.
	Create a file named dump-deallocate-collapse-range-test-<random int> and try to collapse it.
	Remove file after the test.

-t, --truncate
: Truncate FILE (to size 0) at the end of the whole dump.
	It is not recommended since another process can write in FILE between the last read and the truncate call.
	On normal condition, at the end, FILE will size 0.

-r, --remove
: Remove FILE at the end of the whole dump.
	It is not recommended since another process might be using FILE.

## Build

	go get "golang.org/x/sys/unix"
	go test
	go build -o dump-deallocate main.go flag_handling.go dump-deallocate.go

You may have to define GOPATH.
