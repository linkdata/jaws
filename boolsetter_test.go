package jaws

import (
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

var _ BoolSetter = (*testSetter[bool])(nil)

func Test_makeBoolSetter_panic(t *testing.T) {
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
	makeBoolSetter(uint32(1))
}

func Test_makeBoolSetter(t *testing.T) {
	val := true
	var av atomic.Value
	av.Store(val)
	ts := newTestSetter(val)

	tests := []struct {
		name string
		v    interface{}
		want BoolSetter
		out  bool
		err  error
		tag  interface{}
	}{
		{
			name: "BoolSetter",
			v:    ts,
			want: ts,
			out:  val,
			tag:  ts,
		},
		{
			name: "bool",
			v:    val,
			want: boolGetter{val},
			out:  val,
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
			got := makeBoolSetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeBoolSetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetBool(nil); out != tt.out {
				t.Errorf("makeBoolSetter().JawsGetBool() = %v, want %v", out, tt.out)
			}
			if err := got.JawsSetBool(nil, !val); err != tt.err {
				t.Errorf("makeBoolSetter().JawsSetBool() = %v, want %v", err, tt.err)
			}
			gotTag := any(got)
			if tg, ok := got.(TagGetter); ok {
				gotTag = tg.JawsGetTag(nil)
			}
			if gotTag != tt.tag {
				t.Errorf("makeBoolSetter().tag = %v, want %v", gotTag, tt.tag)
			}
		})
	}
}
