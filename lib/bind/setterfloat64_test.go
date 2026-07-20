package bind

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

type testGetter[T comparable] struct {
	v T
}

func (tg testGetter[T]) JawsGet(elem *jaws.Element) T {
	return tg.v
}

func Test_makeSetterFloat64types(t *testing.T) {
	tsint := newTestSetter(int(0))
	tests := []struct {
		name  string
		v     any
		wantS Setter[float64]
	}{
		{
			name:  "Setter[float64]",
			v:     setterFloat64[float64]{},
			wantS: setterFloat64[float64]{},
		},
		{
			name:  "Getter[float64]",
			v:     testGetter[float64]{},
			wantS: setterReadOnly[float64]{testGetter[float64]{}},
		},
		{
			name:  "float64",
			v:     float64(0),
			wantS: setterStatic[float64]{0},
		},
		{
			name:  "Setter[int]",
			v:     tsint,
			wantS: setterFloat64[int]{tsint},
		},
		{
			name:  "Getter[int]",
			v:     testGetter[int]{},
			wantS: setterFloat64ReadOnly[int]{testGetter[int]{}},
		},
		{
			name:  "int",
			v:     int(0),
			wantS: setterFloat64Static[int]{0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotS := MakeSetterFloat64(tt.v); !reflect.DeepEqual(gotS, tt.wantS) {
				t.Errorf("makeSetterFloat64() = %#v, want %#v", gotS, tt.wantS)
			}
		})
	}
}

func Test_makeSetterFloat64_int(t *testing.T) {
	tsint := newTestSetter(int(0))
	gotS := MakeSetterFloat64(tsint)
	err := gotS.JawsSet(nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if x := gotS.JawsGet(nil); x != 1 {
		t.Error(x)
	}
	tg := gotS.(tag.TagGetter)
	if x := tg.JawsGetTag(nil); x != tsint {
		t.Error(x)
	}
}

func Test_setterFloat64_conversionSignalsCanonicalChange(t *testing.T) {
	t.Run("int truncation", func(t *testing.T) {
		ts := newTestSetter(1)
		s := MakeSetterFloat64(ts)
		if err := s.JawsSet(nil, 1.9); err != nil {
			t.Fatalf("JawsSet(1.9): %v", err)
		}
		if ts.Get() != 1 {
			t.Fatalf("stored value = %d, want 1", ts.Get())
		}
		if err := s.JawsSet(nil, 1); !errors.Is(err, jaws.ErrValueUnchanged) {
			t.Fatalf("JawsSet(1) = %v, want ErrValueUnchanged", err)
		}
	})

	t.Run("float32 rounding", func(t *testing.T) {
		value := float32(0.1)
		ts := newTestSetter(value)
		s := MakeSetterFloat64(ts)
		if err := s.JawsSet(nil, 0.1); err != nil {
			t.Fatalf("JawsSet(0.1): %v", err)
		}
		if ts.Get() != value {
			t.Fatalf("stored value = %v, want %v", ts.Get(), value)
		}
		if err := s.JawsSet(nil, float64(value)); !errors.Is(err, jaws.ErrValueUnchanged) {
			t.Fatalf("JawsSet(canonical value) = %v, want ErrValueUnchanged", err)
		}
	})
}

// Test_setterFloat64_sanitizesUntrustedInput covers the float-from-client guard:
// non-finite values are rejected for every numeric type and out-of-range values
// are rejected before the (otherwise wrapping) float->int conversion. The bound
// value must be left unchanged on rejection.
func Test_setterFloat64_sanitizesUntrustedInput(t *testing.T) {
	t.Run("rejects NaN and Inf", func(t *testing.T) {
		ts := newTestSetter(int8(5))
		s := MakeSetterFloat64(ts)
		for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			if err := s.JawsSet(nil, bad); !errors.Is(err, ErrFloatNotFinite) {
				t.Errorf("JawsSet(%v): expected ErrFloatNotFinite, got %v", bad, err)
			}
		}
		if ts.Get() != 5 {
			t.Errorf("bound value mutated to %v", ts.Get())
		}
	})

	t.Run("rejects out-of-range integer conversions", func(t *testing.T) {
		ts := newTestSetter(int8(5))
		s := MakeSetterFloat64(ts)
		for _, bad := range []float64{128, -129, 1e9} { // int8 range is [-128, 127]
			if err := s.JawsSet(nil, bad); !errors.Is(err, ErrFloatOutOfRange) {
				t.Errorf("JawsSet(%v): expected ErrFloatOutOfRange, got %v", bad, err)
			}
		}
		if ts.Get() != 5 {
			t.Errorf("bound value mutated to %v", ts.Get())
		}
		if err := s.JawsSet(nil, 100); err != nil { // in range
			t.Fatalf("JawsSet(100): %v", err)
		}
		if ts.Get() != 100 {
			t.Errorf("want 100 got %v", ts.Get())
		}
	})

	t.Run("rejects 2^63 overflow for int without float64 rounding false-positive", func(t *testing.T) {
		ts := newTestSetter(int(7))
		s := MakeSetterFloat64(ts)
		if err := s.JawsSet(nil, 9223372036854775808.0); !errors.Is(err, ErrFloatOutOfRange) { // 2^63
			t.Errorf("expected ErrFloatOutOfRange, got %v", err)
		}
		if ts.Get() != 7 {
			t.Errorf("bound value mutated to %v", ts.Get())
		}
		// A value safely below the top of int's range must be accepted, proving the
		// 2^63 rejection is not a float64-rounding false positive. int's width is
		// word-size-dependent, so probe with a value that fits the target int.
		inRange := float64(1 << 62) // safely inside a 64-bit int
		if strconv.IntSize == 32 {
			inRange = 1 << 30 // safely inside a 32-bit int
		}
		if err := s.JawsSet(nil, inRange); err != nil {
			t.Fatalf("JawsSet(%v): %v", inRange, err)
		}
	})
}

// assertIntTypeGuard exercises sanitizeFloatForT's per-type branch for an integer
// type T: NaN/Inf is rejected, an out-of-range value overflows, and an in-range
// value is accepted.
func assertIntTypeGuard[T numeric](t *testing.T, name string, inRange, tooBig float64) {
	t.Helper()
	s := MakeSetterFloat64(newTestSetter(T(0)))
	if err := s.JawsSet(nil, math.Inf(1)); !errors.Is(err, ErrFloatNotFinite) {
		t.Errorf("%s: Inf: got %v, want ErrFloatNotFinite", name, err)
	}
	if err := s.JawsSet(nil, tooBig); !errors.Is(err, ErrFloatOutOfRange) {
		t.Errorf("%s: %v: got %v, want ErrFloatOutOfRange", name, tooBig, err)
	}
	if err := s.JawsSet(nil, inRange); err != nil {
		t.Errorf("%s: %v: got %v, want nil", name, inRange, err)
	}
}

// Test_setterFloat64_coversNumericTypes exercises every case of the type switch in
// sanitizeFloatForT: each integer type rejects out-of-range values, float32 rejects
// non-finite and finite-but-overflowing values, and float64 takes the
// finiteness-only default case.
func Test_setterFloat64_coversNumericTypes(t *testing.T) {
	assertIntTypeGuard[int8](t, "int8", 1, 128)
	assertIntTypeGuard[int16](t, "int16", 1, 32768)
	assertIntTypeGuard[int32](t, "int32", 1, 2147483648)
	assertIntTypeGuard[int64](t, "int64", 1, 9223372036854775808.0) // 2^63
	assertIntTypeGuard[int](t, "int", 1, 9223372036854775808.0)     // 2^63
	assertIntTypeGuard[uint8](t, "uint8", 1, 256)
	assertIntTypeGuard[uint16](t, "uint16", 1, 65536)
	assertIntTypeGuard[uint32](t, "uint32", 1, 4294967296)             // 2^32
	assertIntTypeGuard[uint64](t, "uint64", 1, 18446744073709551616.0) // 2^64
	assertIntTypeGuard[uint](t, "uint", 1, 18446744073709551616.0)     // 2^64

	// float32 rejects non-finite values and finite values that overflow the float32
	// range (they would otherwise convert to ±Inf); an in-range value is accepted.
	fs := MakeSetterFloat64(newTestSetter(float32(0)))
	if err := fs.JawsSet(nil, math.Inf(1)); !errors.Is(err, ErrFloatNotFinite) {
		t.Errorf("float32 Inf: got %v, want ErrFloatNotFinite", err)
	}
	if err := fs.JawsSet(nil, 1e30); err != nil {
		t.Errorf("float32 1e30: got %v, want nil", err)
	}
	if err := fs.JawsSet(nil, 1e40); !errors.Is(err, ErrFloatOutOfRange) {
		t.Errorf("float32 1e40: got %v, want ErrFloatOutOfRange", err)
	}
	if err := fs.JawsSet(nil, -1e40); !errors.Is(err, ErrFloatOutOfRange) {
		t.Errorf("float32 -1e40: got %v, want ErrFloatOutOfRange", err)
	}
}

// Test_setterFloat64_intBoundsTrackWordSize pins that the int and uint guards
// follow the target word size rather than assuming 64 bits. 2^31 fits a 64-bit
// int but overflows a 32-bit int, and 2^32 fits a 64-bit uint but overflows a
// 32-bit uint; on a 32-bit build these must be rejected instead of silently
// wrapping in the float->int conversion, and on a 64-bit build they stay in range.
func Test_setterFloat64_intBoundsTrackWordSize(t *testing.T) {
	intErr := sanitizeFloatForT[int](1 << 31)   // 2^31 = MaxInt32 + 1
	uintErr := sanitizeFloatForT[uint](1 << 32) // 2^32 = MaxUint32 + 1
	if strconv.IntSize == 32 {
		if !errors.Is(intErr, ErrFloatOutOfRange) {
			t.Errorf("32-bit int 2^31: got %v, want ErrFloatOutOfRange", intErr)
		}
		if !errors.Is(uintErr, ErrFloatOutOfRange) {
			t.Errorf("32-bit uint 2^32: got %v, want ErrFloatOutOfRange", uintErr)
		}
		return
	}
	if intErr != nil {
		t.Errorf("64-bit int 2^31: got %v, want nil", intErr)
	}
	if uintErr != nil {
		t.Errorf("64-bit uint 2^32: got %v, want nil", uintErr)
	}
}

// Test_setterFloat64_truncationBoundarySymmetry covers the lower-bound truncation
// tolerance: a fractional value just past MinIntN (e.g. -128.5 for int8) truncates
// toward zero to the valid MinIntN and must be accepted, exactly as the mirror-image
// value just past MaxIntN (e.g. 127.5 -> 127) already is. A whole value one below
// MinIntN must still be rejected, and MinInt64 itself must remain acceptable.
func Test_setterFloat64_truncationBoundarySymmetry(t *testing.T) {
	assertSym := func(t *testing.T, name string, errHi, errLo error) {
		t.Helper()
		if errHi != nil {
			t.Errorf("%s: high boundary rejected: %v", name, errHi)
		}
		if errLo != nil {
			t.Errorf("%s: low boundary rejected (truncates to a valid value): %v", name, errLo)
		}
	}
	assertSym(t, "int8", sanitizeFloatForT[int8](math.MaxInt8+0.5), sanitizeFloatForT[int8](math.MinInt8-0.5))
	assertSym(t, "int16", sanitizeFloatForT[int16](math.MaxInt16+0.5), sanitizeFloatForT[int16](math.MinInt16-0.5))
	assertSym(t, "int32", sanitizeFloatForT[int32](math.MaxInt32+0.5), sanitizeFloatForT[int32](math.MinInt32-0.5))

	// A whole value one below MinIntN truncates out of range and must be rejected.
	if err := sanitizeFloatForT[int8](math.MinInt8 - 1); !errors.Is(err, ErrFloatOutOfRange) {
		t.Errorf("int8 %v: got %v, want ErrFloatOutOfRange", math.MinInt8-1, err)
	}
	// MinInt64 is exactly representable and in range; it must remain acceptable, while
	// the next float64 below it (2048 lower) is out of range.
	if err := sanitizeFloatForT[int64](math.MinInt64); err != nil {
		t.Errorf("int64 MinInt64: got %v, want nil", err)
	}
	if err := sanitizeFloatForT[int64](math.Nextafter(math.MinInt64, math.Inf(-1))); !errors.Is(err, ErrFloatOutOfRange) {
		t.Errorf("int64 below MinInt64: got %v, want ErrFloatOutOfRange", err)
	}

	// The fix is observable through the public JawsSet path: -128.5 now stores -128
	// instead of being rejected, matching how 127.5 stores 127.
	ts := newTestSetter(int8(5))
	s := MakeSetterFloat64(ts)
	if err := s.JawsSet(nil, -128.5); err != nil {
		t.Fatalf("JawsSet(-128.5): %v", err)
	}
	if ts.Get() != -128 {
		t.Errorf("JawsSet(-128.5) stored %d, want -128", ts.Get())
	}
	if err := s.JawsSet(nil, 127.5); err != nil {
		t.Fatalf("JawsSet(127.5): %v", err)
	}
	if ts.Get() != 127 {
		t.Errorf("JawsSet(127.5) stored %d, want 127", ts.Get())
	}
}

func Test_makeSetterFloat64ReadOnly_int(t *testing.T) {
	tgint := testGetter[int]{1}
	gotS := MakeSetterFloat64(tgint)
	err := gotS.JawsSet(nil, 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if x := gotS.JawsGet(nil); x != 1 {
		t.Error(x)
	}
	tg := gotS.(tag.TagGetter)
	if x := tg.JawsGetTag(nil); x != tgint {
		t.Error(x)
	}
}

func Test_makeSetterFloat64Static_int(t *testing.T) {
	v := 1
	gotS := MakeSetterFloat64(v)
	err := gotS.JawsSet(nil, 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if x := gotS.JawsGet(nil); x != 1 {
		t.Error(x)
	}
	tg := gotS.(tag.TagGetter)
	if x := tg.JawsGetTag(nil); x != nil {
		t.Error(x)
	}
}

func Test_makeSetterFloat64_panic(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Error("expected panic")
		} else if !strings.Contains(x.(error).Error(), "bind.Setter") {
			t.Fatalf("panic = %v, want bind.Setter", x)
		}
	}()

	_ = MakeSetterFloat64("x")
	t.Fatal("expected panic")
}

// Test_makeSetterFloat64_panicNamedNumeric pins the documented contract that
// named (defined) numeric types are matched by exact type only: neither a value
// of such a type nor a Setter over it bridges to float64, and passing one panics.
// This guards against a future switch to underlying-type (~) matching that would
// silently accept them.
func Test_makeSetterFloat64_panicNamedNumeric(t *testing.T) {
	type Celsius float64

	assertPanics := func(name string, v any) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if x := recover(); x == nil {
					t.Error("expected panic")
				} else if !strings.Contains(x.(error).Error(), "bind.Setter") {
					t.Fatalf("panic = %v, want bind.Setter", x)
				}
			}()
			_ = MakeSetterFloat64(v)
			t.Fatal("expected panic")
		})
	}

	assertPanics("value", Celsius(20))
	assertPanics("setter", newTestSetter[Celsius](20))
}
