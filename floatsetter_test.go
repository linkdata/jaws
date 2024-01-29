package jaws

import (
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

var _ FloatSetter = (*testSetter[float64])(nil)

func Test_makeFloatSetter_panic(t *testing.T) {
	defer func() {
		if x := recover(); x != nil {
			if err, ok := x.(error); ok {
				if strings.Contains(err.Error(), "string") {
					return
				}
			}
		}
		t.Fail()
	}()
	makeFloatSetter("meh")
}

func Test_makeFloatSetter(t *testing.T) {
	val := float64(12.34)
	var av atomic.Value
	av.Store(val)
	ts := newTestSetter(val)

	tests := []struct {
		name string
		v    any
		want FloatSetter
		out  float64
		err  error
		tag  any
	}{
		{
			name: "FloatSetter",
			v:    ts,
			want: ts,
			out:  val,
			tag:  ts,
		},
		{
			name: "float64",
			v:    val,
			want: floatGetter{val},
			out:  val,
			err:  ErrValueNotSettable,
			tag:  nil,
		},
		{
			name: "float32",
			v:    float32(val),
			want: floatGetter{float64(float32(val))},
			out:  float64(float32(val)),
			err:  ErrValueNotSettable,
			tag:  nil,
		},
		{
			name: "int",
			v:    int(val),
			want: floatGetter{float64(int(val))},
			out:  float64(int(val)),
			err:  ErrValueNotSettable,
			tag:  nil,
		},
		{
			name: "*atomic.Value",
			v:    &av,
			want: atomicSetter{&av},
			out:  val,
			tag:  &av,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeFloatSetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeFloatSetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetFloat(nil); out != tt.out {
				t.Errorf("makeFloatSetter().JawsGetFloat() = %v, want %v", out, tt.out)
			}
			if err := got.JawsSetFloat(nil, -val); err != tt.err {
				t.Errorf("makeFloatSetter().JawsSetFloat() = %v, want %v", err, tt.err)
			}
			gotTag := any(got)
			if tg, ok := got.(TagGetter); ok {
				gotTag = tg.JawsGetTag(nil)
			}
			if gotTag != tt.tag {
				t.Errorf("makeFloatSetter().tag = %v, want %v", gotTag, tt.tag)
			}

		})
	}
}
