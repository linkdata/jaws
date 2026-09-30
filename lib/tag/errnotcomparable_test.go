package tag

import (
	"errors"
	"math"
	"testing"
)

type testRuntimeNonComparable struct {
	v any
}

func Test_newErrNotComparable_Error(t *testing.T) {
	err := NewErrNotComparable([]int{1, 2, 3})
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
	err := NewErrNotComparable(testRuntimeNonComparable{v: func() {}})
	if !errors.Is(err, ErrNotComparable) {
		t.Fatalf("expected ErrNotComparable, got %v", err)
	}
}

func TestNewErrNotComparable_ZeroLengthNonComparableArray(t *testing.T) {
	bad := [0]struct{ values []int }{}
	for _, value := range []any{
		bad,
		struct{ value any }{bad},
		[1]any{bad},
		struct {
			nan   float64
			value any
		}{math.NaN(), bad},
	} {
		if err := NewErrNotComparable(value); !errors.Is(err, ErrNotComparable) {
			t.Errorf("NewErrNotComparable(%T) = %v, want ErrNotComparable", value, err)
		}
	}
}

func TestNewErrNotComparable_NaN(t *testing.T) {
	for _, value := range []any{math.NaN(), struct{ value float64 }{math.NaN()}} {
		if err := NewErrNotComparable(value); err != nil {
			t.Errorf("NewErrNotComparable(%T) = %v, want nil", value, err)
		}
	}
}
