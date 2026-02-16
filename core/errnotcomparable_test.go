package core

import (
	"errors"
	"testing"
)

func Test_newErrNotComparable_Error(t *testing.T) {
	err := newErrNotComparable([]int{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for non-comparable value")
	}

	const want = "[]int is not comparable"
	if got := err.Error(); got != want {
		t.Fatalf("unexpected error text %q, want %q", got, want)
	}

	var typed errNotComparable
	if !errors.As(err, &typed) {
		t.Fatalf("expected errNotComparable, got %T", err)
	}
	if got := typed.Error(); got != want {
		t.Fatalf("unexpected typed error text %q, want %q", got, want)
	}
}

