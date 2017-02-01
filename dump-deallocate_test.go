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
		if return_v := BoolToInt(true); return_v != 1 {
			t.Errorf("got '%v'; expected '%v'", return_v, 1)
		}
	})

	t.Run("false", func(t *testing.T) {
		if return_v := BoolToInt(false); return_v != 0 {
			t.Errorf("got '%v'; expected '%v'", return_v, 0)
		}
	})
}

func TestGetFilesystemBlockSize(t *testing.T) {
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
	filesystem_block_size := GetFilesystemBlockSize(file)

	if filesystem_block_size <= 0 {
		t.Errorf("invalide filesystem block size returned : '%v'", filesystem_block_size)
	}
}

func TestCopyWhileDeallocate(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error("Panic : ", r)
		}
	}()

	// get content from LICENSE file
	test_content, err := ioutil.ReadFile("LICENSE")
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
	_, err = file.Write(test_content)
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
	var file_info_before unix.Stat_t
	err = unix.Fstat(int(file.Fd()), &file_info_before)
	if err != nil {
		t.Fatal(err)
	}

	// buffer should be feed with the content of file (LICENSE)
	// and file should be deallocated
	output_buffer := new(bytes.Buffer)
	CopyWhileDeallocate(file, output_buffer)

	// check if buffer has been feed with content of file (LICENSE)
	if !bytes.Equal(test_content, output_buffer.Bytes()) {
		t.Errorf("content hasn't been copied correctly, see '%s'", file.Name())
	}

	// check if file now only contain \0
	_, err = file.Seek(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var file_new_content []byte
	file_new_content, err = ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(file_new_content, []byte{0}) != len(file_new_content) {
		t.Errorf("file should only contain \\0, see '%s'", file.Name())
	}

	// get file stats after the CopyWhileDeallocate
	var file_info_after unix.Stat_t
	err = unix.Fstat(int(file.Fd()), &file_info_after)
	if err != nil {
		t.Fatal(err)
	}

	// check if file has been deallocated
	if file_info_before.Size != file_info_after.Size {
		t.Errorf("file size, expected: '%v', got '%v', see '%s'", file_info_before.Size, file_info_after.Size, file.Name())
	}
	if file_info_before.Blocks <= file_info_after.Blocks {
		t.Errorf("file blocks, expected: < '%v', got '%v', see '%s'", file_info_before.Blocks, file_info_after.Blocks, file.Name())
	}
}

func TestCollapseFileStart(t *testing.T) {
	var err error
	var file *os.File
	var fs_block_size int64

	// check TestCollapse
	defer func() {
		if r := recover(); r != nil {
			t.Error("Panic: ", r)
		}
	}()
	testcollapse_err := TestCollapse()
	if testcollapse_err != nil && testcollapse_err != unix.EOPNOTSUPP {
		t.Fatal(err)
	}

	// get fs block size
	file, err = ioutil.TempFile(".", "dump-deallocate-TestCollapseFileStart-")
	if err != nil {
		t.Fatal(err)
	}

	fs_block_size = GetFilesystemBlockSize(file)

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

	var byte_actualy_deallocated int64

	test_cases := []struct {
		name                string
		file_size           int64
		bytes_to_deallocate int64
		expected_v          int64
		expected_e          error
	}{
		{"1fsb|-1", fs_block_size, -1, 0, error_zero},
		{"1fsb|1", fs_block_size, 1, 0, error_less_than_one_fsb},
		{"2fsb|1fsb", 2 * fs_block_size, fs_block_size, fs_block_size, testcollapse_err},
		{"2fsb|1.5fsb", 2 * fs_block_size, fs_block_size + fs_block_size/2, fs_block_size, testcollapse_err},
	}

	// check CollapseFileStart
	for _, tc := range test_cases {
		t.Run(tc.name, func(t *testing.T) {

			createTestFile(tc.file_size)
			defer os.Remove(file.Name())
			defer file.Close()

			defer func() {
				if r := recover(); r != nil {
					t.Error("Panic: ", r)
				}
			}()

			byte_actualy_deallocated, err = CollapseFileStart(file, tc.bytes_to_deallocate)

			if err != tc.expected_e {
				t.Fatalf("expected error %v, got err: %v", tc.expected_e, err)
			}

			if byte_actualy_deallocated != tc.expected_v {
				t.Errorf("expected: %d, got: %d",
					tc.expected_v,
					byte_actualy_deallocated)
			}
		})
	}
}
