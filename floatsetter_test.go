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

	tests := []struct {
		name string
		v    interface{}
		want FloatSetter
		out  float64
		tag  interface{}
	}{
		{
			name: "FloatSetter",
			v:    floatGetter{val},
			want: floatGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "float64",
			v:    val,
			want: floatGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "float32",
			v:    float32(val),
			want: floatGetter{float64(float32(val))},
			out:  float64(float32(val)),
			tag:  nil,
		},
		{
			name: "int",
			v:    int(val),
			want: floatGetter{float64(int(val))},
			out:  float64(int(val)),
			tag:  nil,
		},
		{
			name: "*atomic.Value",
			v:    &av,
			want: atomicGetter{&av},
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
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeFloatSetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
