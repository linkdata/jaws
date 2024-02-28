package jaws

import (
	"html/template"
	"reflect"
	"strings"
	"testing"
)

var _ StringGetter = (*testSetter[string])(nil)

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
	ts := newTestSetter(val)

	tests := []struct {
		name string
		v    any
		want StringGetter
		out  string
		err  error
		tag  any
	}{
		{
			name: "StringGetter",
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
			name: "template.HTMLAttr",
			v:    template.HTMLAttr(val),
			want: stringGetter{val},
			out:  val,
			err:  ErrValueNotSettable,
			tag:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeStringGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeStringGetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetString(nil); out != tt.out {
				t.Errorf("makeStringGetter().JawsGetString() = %v, want %v", out, tt.out)
			}
			gotTag := any(got)
			if tg, ok := got.(TagGetter); ok {
				gotTag = tg.JawsGetTag(nil)
			}
			if gotTag != tt.tag {
				t.Errorf("makeStringGetter().tag = %v, want %v", gotTag, tt.tag)
			}
		})
	}
}
