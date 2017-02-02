package main

import (
	"bytes"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"os"
	"testing"
)

func TestBoolToInt(t *testing.T) {

	t.Run("true", func(t *testing.T) {
		if returnV := BoolToInt(true); returnV != 1 {
			t.Errorf("got '%v'; expected '%v'", returnV, 1)
		}
	})

	t.Run("false", func(t *testing.T) {
		if returnV := BoolToInt(false); returnV != 0 {
			t.Errorf("got '%v'; expected '%v'", returnV, 0)
		}
	})
}

func TestFilesystemBlockSize(t *testing.T) {
	// create the test file
	file, err := os.Open("LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Error("Panic: ", r)
		}
	}()
	filesystemBlockSize := FilesystemBlockSize(file)

	if filesystemBlockSize <= 0 {
		t.Errorf("invalide filesystem block size returned : '%v'", filesystemBlockSize)
	}
}

func TestCopyWhileDeallocate(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error("Panic : ", r)
		}
	}()

	// get content from LICENSE file
	testContent, err := ioutil.ReadFile("LICENSE")
	if err != nil {
		t.Fatal(err)
	}

	// create the test file
	file, err := ioutil.TempFile(".", "dump-deallocate-TestCopyWhileDeallocate-")
	if err != nil {
		t.Fatal(err)
	}

	// keep the file if the test fail
	defer func() {
		if !t.Failed() {
			os.Remove(file.Name())
		}
	}()
	defer file.Close()

	// write the test content (LICENSE) on the test file
	_, err = file.Write(testContent)
	if err != nil {
		t.Fatal(err)
	}

	// sync (just to be sure)
	err = file.Sync()
	if err != nil {
		t.Fatal(err)
	}

	// seek to the begining of the file
	_, err = file.Seek(0, 0)
	if err != nil {
		t.Fatal(err)
	}

	// get file stats before the CopyWhileDeallocate
	var fileInfoBefore unix.Stat_t
	err = unix.Fstat(int(file.Fd()), &fileInfoBefore)
	if err != nil {
		t.Fatal(err)
	}

	// buffer should be feed with the content of file (LICENSE)
	// and file should be deallocated
	outputBuffer := new(bytes.Buffer)
	CopyWhileDeallocate(file, outputBuffer)

	// check if buffer has been feed with content of file (LICENSE)
	if !bytes.Equal(testContent, outputBuffer.Bytes()) {
		t.Errorf("content hasn't been copied correctly, see '%s'", file.Name())
	}

	// check if file now only contain \0
	_, err = file.Seek(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fileNewContent []byte
	fileNewContent, err = ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(fileNewContent, []byte{0}) != len(fileNewContent) {
		t.Errorf("file should only contain \\0, see '%s'", file.Name())
	}

	// get file stats after the CopyWhileDeallocate
	var fileInfoAfter unix.Stat_t
	err = unix.Fstat(int(file.Fd()), &fileInfoAfter)
	if err != nil {
		t.Fatal(err)
	}

	// check if file has been deallocated
	if fileInfoBefore.Size != fileInfoAfter.Size {
		t.Errorf("file size, expected: '%v', got '%v', see '%s'", fileInfoBefore.Size, fileInfoAfter.Size, file.Name())
	}
	if fileInfoBefore.Blocks <= fileInfoAfter.Blocks {
		t.Errorf("file blocks, expected: < '%v', got '%v', see '%s'", fileInfoBefore.Blocks, fileInfoAfter.Blocks, file.Name())
	}
}

func TestCollapseFileStart(t *testing.T) {
	var err error
	var file *os.File
	var fsBlockSize int64

	// check TestCollapse
	defer func() {
		if r := recover(); r != nil {
			t.Error("Panic: ", r)
		}
	}()
	testCollapseErr := TestCollapse()
	if testCollapseErr != nil && testCollapseErr != unix.EOPNOTSUPP {
		t.Fatal(err)
	}

	// get fs block size
	file, err = ioutil.TempFile(".", "dump-deallocate-TestCollapseFileStart-")
	if err != nil {
		t.Fatal(err)
	}

	fsBlockSize = FilesystemBlockSize(file)

	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = os.Remove(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	// create function used on each test
	createTestFile := func(size int64) {

		file, err = ioutil.TempFile(".", "dump-deallocate-TestCollapseFileStart-")
		if err != nil {
			t.Fatal(err)
		}

		// mode 0 = Default: allocate disk space
		err = unix.Fallocate(int(file.Fd()), 0, 0, size)
		if err != nil {
			t.Fatal(err)
		}
	}

	var byteActualyDeallocated int64

	testCases := []struct {
		name                string
		fileSize           int64
		bytesToDeallocate int64
		expectedV          int64
		expectedE          error
	}{
		{"1fsb|-1", fsBlockSize, -1, 0, errorZero},
		{"1fsb|1", fsBlockSize, 1, 0, errorLessThanOneFsb},
		{"2fsb|1fsb", 2 * fsBlockSize, fsBlockSize, fsBlockSize, testCollapseErr},
		{"2fsb|1.5fsb", 2 * fsBlockSize, fsBlockSize + fsBlockSize/2, fsBlockSize, testCollapseErr},
	}

	// check CollapseFileStart
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			createTestFile(tc.fileSize)
			defer os.Remove(file.Name())
			defer file.Close()

			defer func() {
				if r := recover(); r != nil {
					t.Error("Panic: ", r)
				}
			}()

			byteActualyDeallocated, err = CollapseFileStart(file, tc.bytesToDeallocate)

			if err != tc.expectedE {
				t.Fatalf("expected error %v, got err: %v", tc.expectedE, err)
			}

			if byteActualyDeallocated != tc.expectedV {
				t.Errorf("expected: %d, got: %d",
					tc.expectedV,
					byteActualyDeallocated)
			}
		})
	}
}
