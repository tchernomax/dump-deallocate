package main

import (
	"flag"
	"strconv"
	"strings"
	"testing"
)

// check if default buffer size is a multiple of 1024
func TestDefaultBufferSize(t *testing.T) {
	if bufferSize%1024 != 0 {
		t.Error("bufferSize (default buffer size) should be a multiple of 1024")
	}
}

func TestBufferSizeParsing(t *testing.T) {
	bufferSize := new(sizeType)

	testCases := []struct {
		inputV    string
		expectedV int64
		expectedE error
	}{
		{"10",      10,      nil},
		{"10KiB",   10*1024, nil},
		{"10KB",    10*1000, nil},
		{"-1",      0,       errorNegativeOrZero},
		{"-10KB",   0,       errorNegativeOrZero},
		{"0",       0,       errorNegativeOrZero},
		{"1024EiB", 0,       errorInt64Overflow},
		{"KB",      0,       strconv.ErrSyntax},
		{"test",    0,       strconv.ErrSyntax},
		{"10000000000000000000000", 0, strconv.ErrRange},
	}

	for _, tc := range testCases {
		t.Run(tc.inputV, func(t *testing.T) {

			err := bufferSize.Set(tc.inputV)

			// check error
			if err != tc.expectedE {
				t.Errorf("got error '%v'; expected error '%v'", err, tc.expectedE)
			}

			if err != nil {
				// if we expected an error we don't check the value
				return
			}

			// check value
			if int64(*bufferSize) != tc.expectedV {
				t.Errorf("got '%d'; expected '%v'", bufferSize, tc.expectedV)
			}
		})
	}
}

func TestPostParsingCheckFlags(t *testing.T) {
	var err error

	testCases := []struct {
		inputV    []string
		expectedE error
	}{
		{[]string{"test"},             nil},
		{[]string{"-C"},               nil},
		{[]string{"-c", "test"},       nil},
		{[]string{"-t", "test"},       nil},
		{[]string{"-r", "test"},       nil},
		{[]string{"-C", "test"},       errorHaveFile},
		{[]string{"-c"},               errorMissingFile},
		{[]string{"-t"},               errorMissingFile},
		{[]string{"-r"},               errorMissingFile},
		{[]string{"-c", "-t", "test"}, errorMutuallyExclusive},
		{[]string{"-c", "-r", "test"}, errorMutuallyExclusive},
		{[]string{"-t", "-r", "test"}, errorMutuallyExclusive},
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc.inputV, " "), func(t *testing.T) {
			// reset the flags
			collapse, collapseTest, truncate, remove = collapseDefault, collapseTestDefault, truncateDefault, removeDefault

			// parse the input
			flag.CommandLine.Parse(tc.inputV)

			err = PostParsingCheckFlags()
			if err != tc.expectedE {
				t.Errorf("expected error: %v, got: %v", tc.expectedE, err)
			}
		})
	}
}
