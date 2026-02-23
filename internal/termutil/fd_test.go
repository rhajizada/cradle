package termutil_test

import (
	"testing"

	"github.com/rhajizada/cradle/internal/termutil"
)

func TestIntValidFD(t *testing.T) {
	got, ok := termutil.Int(uintptr(42))
	if !ok {
		t.Fatalf("expected valid fd conversion")
	}
	if got != 42 {
		t.Fatalf("unexpected fd value: %d", got)
	}
}

func TestIntOverflowFD(t *testing.T) {
	overflow := uintptr(termutil.MaxInt) + 1
	_, ok := termutil.Int(overflow)
	if ok {
		t.Fatalf("expected overflow fd conversion to fail")
	}
}
