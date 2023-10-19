package jaws

import (
	"html/template"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

var _ StringSetter = (*testSetter[string])(nil)

func Test_makeStringSetter_panic(t *testing.T) {
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
	makeStringSetter(uint32(42))
}

func Test_makeStringSetter(t *testing.T) {
	val := "<span>"
	var av atomic.Value
	av.Store(val)

	ts := newTestSetter(val)

	tests := []struct {
		name string
		v    interface{}
		want StringSetter
		out  string
		err  error
		tag  interface{}
	}{
		{
			name: "StringSetter",
			v:    ts,
			want: ts,
			out:  val,
			tag:  ts,
		},
		{
			name: "string",
			v:    val,
			want: stringGetter{val},
			out:  val,
			err:  ErrValueNotSettable,
			tag:  nil,
		},
		{
			name: "template.HTML",
			v:    template.HTML(val),
			want: stringGetter{val},
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
			got := makeStringSetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeStringSetter() = %v, want %v", got, tt.want)
			}
			if txt := got.JawsGetString(nil); txt != tt.out {
				t.Errorf("makeStringSetter().JawsGetString() = %v, want %v", txt, tt.out)
			}
			if err := got.JawsSetString(nil, "str"); err != tt.err {
				t.Errorf("makeStringSetter().JawsSetString() = %v, want %v", err, tt.err)
			}
			var gotTag any
			if tg, ok := got.(TagGetter); ok {
				gotTag = tg.JawsGetTag(nil)
			} else {
				gotTag = got
			}
			if gotTag != tt.tag {
				t.Errorf("makeStringSetter().JawsGetTag() = %v, want %v", gotTag, tt.tag)
			}
		})
	}
}
