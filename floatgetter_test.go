package jaws

import (
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func Test_makeFloatGetter_panic(t *testing.T) {
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
	makeFloatGetter("meh")
}

func Test_makeFloatGetter(t *testing.T) {
	val := float64(12.34)
	var av atomic.Value
	av.Store(val)

	tests := []struct {
		name string
		v    interface{}
		want FloatGetter
		out  float64
		tag  interface{}
	}{
		{
			name: "FloatGetter",
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
			got := makeFloatGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeFloatGetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetFloat(nil); out != tt.out {
				t.Errorf("makeFloatGetter().JawsGetFloat() = %v, want %v", out, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeFloatGetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
