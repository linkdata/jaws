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

type simpleGetter struct {
	Value string
}

func (g simpleGetter) JawsGetString(e *Element) string {
	return g.Value
}

type simpleGetterT struct {
	Value string
}

func (g simpleGetterT) JawsGet(e *Element) string {
	return g.Value
}

type simpleSetterT struct {
	Value string
}

func (g simpleSetterT) JawsGet(e *Element) string {
	return g.Value
}

func (g simpleSetterT) JawsSet(e *Element, v string) error {
	return nil
}

type simpleNotagGetter struct {
	v string
}

func (g simpleNotagGetter) JawsGetString(e *Element) string {
	return g.v
}

func (g simpleNotagGetter) JawsGetTag(rq *Request) any {
	return nil
}

func Test_makeStringSetter(t *testing.T) {
	val := "<span>"
	var av atomic.Value
	av.Store(val)
	ts := newTestSetter(val)
	stringer := testStringer{}

	sg := simpleGetter{Value: val}
	sgt := simpleGetterT{Value: val}
	sst := simpleSetterT{Value: val}
	sng := simpleNotagGetter{v: val}

	tests := []struct {
		name string
		v    any
		want StringSetter
		out  string
		err  error
		tag  any
	}{
		{
			name: "StringGetter_untagged",
			v:    sng,
			want: stringSetter{sng},
			out:  val,
			err:  ErrValueNotSettable,
			tag:  nil,
		},
		{
			name: "StringSetter",
			v:    ts,
			want: ts,
			out:  val,
			tag:  ts,
		},
		{
			name: "StringGetter",
			v:    sg,
			want: stringSetter{sg},
			out:  val,
			err:  ErrValueNotSettable,
			tag:  sg,
		},
		{
			name: "StringerGetter",
			v:    stringer,
			want: stringerGetter{stringer},
			out:  testStringer{}.String(),
			err:  ErrValueNotSettable,
			tag:  stringer,
		},
		{
			name: "Setter[string]",
			v:    sst,
			want: stringSetterT{sst},
			out:  val,
			tag:  sst,
		},
		{
			name: "Getter[string]",
			v:    sgt,
			want: stringGetterT{sgt},
			out:  val,
			err:  ErrValueNotSettable,
			tag:  sgt,
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
			if out := got.JawsGetString(nil); out != tt.out {
				t.Errorf("makeStringSetter().JawsGetString() = %v, want %v", out, tt.out)
			}
			if err := got.JawsSetString(nil, "str"); err != tt.err {
				t.Errorf("makeStringSetter().JawsSetString() = %v, want %v", err, tt.err)
			}

			gotTag := MustTagExpand(nil, got)
			if len(gotTag) == 1 {
				if gotTag[0] != tt.tag {
					t.Errorf("makeStringSetter().tag = %v, want %v", gotTag, tt.tag)
				}
			} else if tt.tag != nil {
				t.Error(len(gotTag))
			}
		})
	}
}
