package jaws

import (
	"errors"
	"testing"
)

type testRuntimeNonComparable struct {
	v any
}

func Test_newErrNotComparable_Error(t *testing.T) {
	err := newErrNotComparable([]int{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for non-comparable value")
	}

	const want = "[]int is not comparable"
	if got := err.Error(); got != want {
		t.Fatalf("unexpected error text %q, want %q", got, want)
	}

	if !errors.Is(err, ErrNotComparable) {
		t.Fatalf("expected ErrNotComparable, got %v", err)
	}
}

func Test_newErrNotComparable_RuntimeNonComparable(t *testing.T) {
	err := newErrNotComparable(testRuntimeNonComparable{v: func() {}})
	if !errors.Is(err, ErrNotComparable) {
		t.Fatalf("expected ErrNotComparable, got %v", err)
	}
}
