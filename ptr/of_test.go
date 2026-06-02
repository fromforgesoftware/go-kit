package ptr_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/ptr"
)

func TestOf(t *testing.T) {
	b := true
	pb := ptr.Of(b)
	if pb == nil {
		t.Fatal("unexpected nil conversion")
	}
	if *pb != b {
		t.Fatalf("got %v, want %v", *pb, b)
	}
}

func TestSliceOf(t *testing.T) {
	arr := ptr.SliceOf[int]()
	if len(arr) != 0 {
		t.Fatal("expected zero length")
	}
	arr = ptr.SliceOf(1, 2, 3, 4, 5)
	for i, v := range arr {
		if *v != i+1 {
			t.Fatal("values don't match")
		}
	}
}
