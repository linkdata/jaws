package bind

import (
	"errors"
	"math"
	"reflect"
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
		if err := s.JawsSet(nil, 1<<62); err != nil { // safely in range
			t.Fatalf("JawsSet(2^62): %v", err)
		}
	})
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
