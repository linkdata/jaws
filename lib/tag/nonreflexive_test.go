package tag

import (
	"errors"
	"math"
	"testing"
)

type (
	testFloat32Tag    float32
	testFloat64Tag    float64
	testComplex64Tag  complex64
	testComplex128Tag complex128
)

func TestTagExpandRejectsNonReflexiveTags(t *testing.T) {
	tests := []struct {
		name string
		tag  any
	}{
		{name: "named float32", tag: testFloat32Tag(float32(math.NaN()))},
		{name: "named float64", tag: testFloat64Tag(math.NaN())},
		{name: "named complex64 real", tag: testComplex64Tag(complex(float32(math.NaN()), 0))},
		{name: "named complex64 imaginary", tag: testComplex64Tag(complex(0, float32(math.NaN())))},
		{name: "named complex128 real", tag: testComplex128Tag(complex(math.NaN(), 0))},
		{name: "named complex128 imaginary", tag: testComplex128Tag(complex(0, math.NaN()))},
		{name: "struct", tag: struct{ Value float64 }{Value: math.NaN()}},
		{name: "array", tag: [1]float64{math.NaN()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TagExpand(nil, tt.tag)
			if !errors.Is(err, ErrNotUsableAsTag) {
				t.Fatalf("TagExpand() error = %v, want %v", err, ErrNotUsableAsTag)
			}
			if !errors.Is(err, ErrNotComparable) {
				t.Fatalf("TagExpand() error = %v, want compatibility with %v", err, ErrNotComparable)
			}
			if result != nil {
				t.Fatalf("TagExpand() result = %#v, want nil", result)
			}
		})
	}
}

func TestNewErrNotUsableAsTagRejectsNonReflexiveTags(t *testing.T) {
	tests := []struct {
		name string
		tag  any
	}{
		{name: "named float", tag: testFloat64Tag(math.NaN())},
		{name: "named complex", tag: testComplex128Tag(complex(0, math.NaN()))},
		{name: "struct", tag: struct{ Value float64 }{Value: math.NaN()}},
		{name: "array", tag: [1]float64{math.NaN()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NewErrNotUsableAsTag(tt.tag); !errors.Is(err, ErrNotUsableAsTag) {
				t.Fatalf("NewErrNotUsableAsTag() = %v, want %v", err, ErrNotUsableAsTag)
			}
		})
	}
}

func TestNonReflexiveKindsAcceptFiniteTags(t *testing.T) {
	tests := []any{
		testFloat32Tag(1.25),
		testFloat64Tag(2.5),
		testComplex64Tag(complex(3, 4)),
		testComplex128Tag(complex(5, 6)),
		struct{ Value float64 }{Value: 7.5},
		[1]float64{8.5},
	}

	for _, tag := range tests {
		result, err := TagExpand(nil, tag)
		if err != nil {
			t.Fatalf("TagExpand(%#v) error = %v", tag, err)
		}
		if len(result) != 1 || result[0] != tag {
			t.Fatalf("TagExpand(%#v) = %#v, want the input tag", tag, result)
		}
		if err = NewErrNotUsableAsTag(tag); err != nil {
			t.Fatalf("NewErrNotUsableAsTag(%#v) = %v, want nil", tag, err)
		}
	}
}
