# dump-deallocate

## help

Usage: ./dump-deallocate [options] file

Dump 'file' on stdout and deallocate it in the same time.
More precisely:

1. read 'buffer-size' bytes from 'file'
2. write thoses bytes on stdout
3. deallocate 'buffer-size' bytes from 'file' (fallocate punch-hole) and go back to 1.

Options:

-b, --buffer-size int
: memory buffer size in byte (default 32KiB).

-c, --collapse-range
: At the end of the whole dump, remove/collapse (with fallocate collapse-range) the greatest number of filesystem blocks already dumped.
	On normal condition, at the end, 'file' will size one filesystem block.  
	Supported on ext4 from Linux 3.15.

-t, --truncate
: Truncate the file (to size 0) at the end of the whole dump.
	It is not recommended since another process can write in the file between the last read and the truncate call.
	On normal condition, at the end, 'file' will size 0.

-r, --remove
: Remove the file at the end of the whole dump.
	It is not recommended since another process might be using the file.

## build

	GOPATH=`pwd` go get "golang.org/x/sys/unix"
	GOPATH=`pwd` go build dump-deallocate.go
