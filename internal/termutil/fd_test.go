package termutil

import "testing"

func TestIntValidFD(t *testing.T) {
	got, ok := Int(uintptr(42))
	if !ok {
		t.Fatalf("expected valid fd conversion")
	}
	if got != 42 {
		t.Fatalf("unexpected fd value: %d", got)
	}
}

func TestIntOverflowFD(t *testing.T) {
	overflow := uintptr(maxInt) + 1
	_, ok := Int(overflow)
	if ok {
		t.Fatalf("expected overflow fd conversion to fail")
	}
}
