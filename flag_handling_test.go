package main

import (
	"flag"
	"strconv"
	"strings"
	"testing"
)

// check if default buffer size is a multiple of 1024
func TestDefaultBufferSize(t *testing.T) {
	if buffer_size%1024 != 0 {
		t.Error("buffer_size (default buffer size) should be a multiple of 1024")
	}
}

func TestBufferSizeParsing(t *testing.T) {
	buffer_size := new(size_type)

	test_cases := []struct {
		input_v    string
		expected_v int64
		expected_e error
	}{
		{"10",      10,      nil},
		{"10KiB",   10*1024, nil},
		{"10KB",    10*1000, nil},
		{"-1",      0,       error_negative_or_zero},
		{"-10KB",   0,       error_negative_or_zero},
		{"0",       0,       error_negative_or_zero},
		{"1024EiB", 0,       error_int64_overflow},
		{"KB",      0,       strconv.ErrSyntax},
		{"test",    0,       strconv.ErrSyntax},
		{"10000000000000000000000", 0, strconv.ErrRange},
	}

	for _, tc := range test_cases {
		t.Run(tc.input_v, func(t *testing.T) {

			err := buffer_size.Set(tc.input_v)

			// check error
			if err != tc.expected_e {
				t.Errorf("got error '%v'; expected error '%v'", err, tc.expected_e)
			}

			if err != nil {
				// if we expected an error we don't check the value
				return
			}

			// check value
			if int64(*buffer_size) != tc.expected_v {
				t.Errorf("got '%d'; expected '%v'", buffer_size, tc.expected_v)
			}
		})
	}
}

func TestPostParsingCheckFlags(t *testing.T) {
	var err error

	test_cases := []struct {
		input_v    []string
		expected_e error
	}{
		{[]string{"test"},             nil},
		{[]string{"-C"},               nil},
		{[]string{"-c", "test"},       nil},
		{[]string{"-t", "test"},       nil},
		{[]string{"-r", "test"},       nil},
		{[]string{"-C", "test"},       error_have_file},
		{[]string{"-c"},               error_missing_file},
		{[]string{"-t"},               error_missing_file},
		{[]string{"-r"},               error_missing_file},
		{[]string{"-c", "-t", "test"}, error_mutually_exclusive},
		{[]string{"-c", "-r", "test"}, error_mutually_exclusive},
		{[]string{"-t", "-r", "test"}, error_mutually_exclusive},
	}

	for _, tc := range test_cases {
		t.Run(strings.Join(tc.input_v, " "), func(t *testing.T) {
			// reset the flags
			collapse, collapse_test, truncate, remove = collapse_default, collapse_test_default, truncate_default, remove_default

			// parse the input
			flag.CommandLine.Parse(tc.input_v)

			err = PostParsingCheckFlags()
			if err != tc.expected_e {
				t.Errorf("expected error: %v, got: %v", tc.expected_e, err)
			}
		})
	}
}
