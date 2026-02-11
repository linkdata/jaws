package jaws

import (
	"reflect"
	"testing"
)

type testGetter[T comparable] struct {
	v T
}

func (tg testGetter[T]) JawsGet(*Element) T {
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
	tg := gotS.(TagGetter)
	if x := tg.JawsGetTag(nil); x != tsint {
		t.Error(x)
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
	tg := gotS.(TagGetter)
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
	tg := gotS.(TagGetter)
	if x := tg.JawsGetTag(nil); x != nil {
		t.Error(x)
	}
}

func Test_makeSetterFloat64_panic(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Error("expected panic")
		}
	}()

	_ = MakeSetterFloat64("x")
	t.Fatal("expected panic")
}
