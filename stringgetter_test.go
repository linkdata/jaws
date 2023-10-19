package jaws

import (
	"html/template"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

var _ StringSetter = (*testSetter[string])(nil)

func Test_makeStringGetter_panic(t *testing.T) {
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
	makeStringGetter(uint32(42))
}

func Test_makeStringGetter(t *testing.T) {
	val := "<span>"
	var av atomic.Value
	av.Store(val)

	tests := []struct {
		name string
		v    interface{}
		want StringGetter
		out  string
		tag  interface{}
	}{
		{
			name: "StringGetter",
			v:    stringGetter{val},
			want: stringGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "string",
			v:    val,
			want: stringGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "template.HTML",
			v:    template.HTML(val),
			want: stringGetter{val},
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
			got := makeStringGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeStringGetter() = %v, want %v", got, tt.want)
			}
			if txt := got.JawsGetString(nil); txt != tt.out {
				t.Errorf("makeStringGetter().JawsGetString() = %v, want %v", txt, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeStringGetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
