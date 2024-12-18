package jaws

import (
	"html"
	"html/template"
	"reflect"
	"sync/atomic"
	"testing"
)

type testAnySetter struct {
	Value any
}

func (ag *testAnySetter) JawsGetAny(*Element) any {
	return ag.Value
}

func (ag *testAnySetter) JawsSetAny(e *Element, v any) error {
	ag.Value = v
	return nil
}

func Test_MakeHTMLGetter(t *testing.T) {
	untypedText := "<span>"
	typedText := template.HTML(untypedText)
	escapedTypedText := template.HTML(html.EscapeString(untypedText))
	var avUntyped, avTyped atomic.Value
	avUntyped.Store(untypedText)
	avTyped.Store(typedText)
	stringer := testStringer{}

	getterString := testGetterString{}
	getterAny := &testAnySetter{Value: untypedText}

	tests := []struct {
		name string
		v    any
		want HTMLGetter
		out  template.HTML
		tag  any
	}{
		{
			name: "HTMLGetter",
			v:    htmlGetter{typedText},
			want: htmlGetter{typedText},
			out:  typedText,
			tag:  nil,
		},
		{
			name: "Getter[string]",
			v:    getterString,
			want: htmlGetterString{getterString},
			out:  template.HTML(html.EscapeString(getterString.JawsGet(nil))),
			tag:  getterString,
		},
		{
			name: "Getter[any]",
			v:    getterAny,
			want: htmlGetterAny{getterAny},
			out:  escapedTypedText,
			tag:  getterAny,
		},
		{
			name: "fmt.Stringer",
			v:    stringer,
			want: htmlStringerGetter{stringer},
			out:  template.HTML(html.EscapeString(stringer.String())),
			tag:  stringer,
		},
		{
			name: "template.HTML",
			v:    typedText,
			want: htmlGetter{typedText},
			out:  typedText,
			tag:  nil,
		},
		{
			name: "string",
			v:    untypedText,
			want: htmlGetter{template.HTML(untypedText)},
			out:  template.HTML(untypedText),
			tag:  nil,
		},
		{
			name: "int",
			v:    123,
			want: htmlGetter{template.HTML("123")},
			out:  template.HTML("123"),
			tag:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeHTMLGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeHTMLGetter(%s)\n  got %#v\n want %#v", tt.name, got, tt.want)
			}
			if txt := got.JawsGetHTML(nil); txt != tt.out {
				t.Errorf("MakeHTMLGetter(%s).JawsGetHTML() = %v, want %v", tt.name, txt, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("MakeHTMLGetter(%s).JawsGetTag() = %v, want %v", tt.name, tag, tt.tag)
			}
		})
	}
}
