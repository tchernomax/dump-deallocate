package main
import (
	"testing"
	"strconv"
)

func TestBufferSizeParsing(t *testing.T) {
	buffer_size := new(size_type)

	test_cases := []struct {
		input_v     string
		expected_v  int64
		expected_e  error
	}{
		{"10",      10,      nil},
		{"10KiB",   10*1024, nil},
		{"10KB",    10*1000, nil},
		{"-1",      0, error_negative_or_zero},
		{"-10KB",   0, error_negative_or_zero},
		{"0",       0, error_negative_or_zero},
		{"1024EiB", 0, error_int64_overflow},
		{"KB",      0, strconv.ErrSyntax},
		{"test",    0, strconv.ErrSyntax},
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
