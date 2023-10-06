package jaws

import (
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func Test_makeBoolGetter_panic(t *testing.T) {
	defer func() {
		if x := recover(); x != nil {
			if err, ok := x.(error); ok {
				if strings.Contains(err.Error(), "uint32") {
					return
				}
			}
		}
		t.Fail()
	}()
	makeBoolGetter(uint32(1))
}

func Test_makeBoolGetter(t *testing.T) {
	val := true
	var av atomic.Value
	av.Store(val)

	tests := []struct {
		name string
		v    interface{}
		want BoolGetter
		out  bool
		tag  interface{}
	}{
		{
			name: "BoolGetter",
			v:    boolGetter{val},
			want: boolGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "bool",
			v:    val,
			want: boolGetter{val},
			out:  val,
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
			got := makeBoolGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeBoolGetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetBool(nil); out != tt.out {
				t.Errorf("makeBoolGetter().JawsGetBool() = %v, want %v", out, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeBoolGetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
