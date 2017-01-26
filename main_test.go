package main
import "testing"

func TestSet(t *testing.T){
	buffer_size := new(size_type)
	err := buffer_size.Set("10")
	if int64(*buffer_size) != 10 || err != nil {
		t.Fatal("expected: 10 ; got: ", *buffer_size)
	}

	err = buffer_size.Set("10KiB")
	if int64(*buffer_size) != 10*1024 || err != nil {
		t.Fatal("expected: ", 10*1024, " ; got: ", *buffer_size)
	}

	err = buffer_size.Set("10KB")
	if int64(*buffer_size) != 10*1000 || err != nil {
		t.Fatal("expected: ", 10*1000, " ; got: ", *buffer_size)
	}

	// errors
	err = buffer_size.Set("-1")
	if err == nil {
		t.Fatal("expected: error", " ; got: ", *buffer_size)
	}

	err = buffer_size.Set("0")
	if err == nil {
		t.Fatal("expected: error", " ; got: ", *buffer_size)
	}

	err = buffer_size.Set("KB")
	if err == nil {
		t.Fatal("expected: error", " ; got: ", *buffer_size)
	}

	err = buffer_size.Set("-10KB")
	if err == nil {
		t.Fatal("expected: error", " ; got: ", *buffer_size)
	}

	err = buffer_size.Set("1024EiB")
	if err == nil {
		t.Fatal("expected: error", " ; got: ", *buffer_size)
	}
}
