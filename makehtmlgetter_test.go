package jaws

import (
	"html"
	"html/template"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

var _ HtmlGetter = (*testSetter[template.HTML])(nil)

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

func Test_MakeHtmlGetter(t *testing.T) {
	untypedText := "<span>"
	typedText := template.HTML(untypedText)
	escapedTypedText := template.HTML(html.EscapeString(untypedText))
	var avUntyped, avTyped atomic.Value
	avUntyped.Store(untypedText)
	avTyped.Store(typedText)
	stringer := testStringer{}

	var mu sync.Mutex
	getterHTML := Bind(&mu, &escapedTypedText)
	getterString := Bind(&mu, &untypedText)
	getterAny := &testAnySetter{Value: untypedText}

	tests := []struct {
		name string
		v    any
		want HtmlGetter
		out  template.HTML
		tag  any
	}{
		{
			name: "HtmlGetter",
			v:    htmlGetter{typedText},
			want: htmlGetter{typedText},
			out:  typedText,
			tag:  nil,
		},
		{
			name: "Getter[template.HTML]",
			v:    getterHTML,
			want: htmlGetterHTML{getterHTML},
			out:  escapedTypedText,
			tag:  getterHTML,
		},
		{
			name: "StringGetter",
			v:    stringGetterStatic{untypedText},
			want: htmlGetterStringGetter{stringGetterStatic{untypedText}},
			out:  escapedTypedText,
			tag:  stringGetterStatic{untypedText},
		},
		{
			name: "Getter[string]",
			v:    getterString,
			want: htmlGetterString{getterString},
			out:  escapedTypedText,
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
			name: "StringerGetter",
			v:    stringer,
			want: htmlGetterStringGetter{stringerGetter{stringer}},
			out:  template.HTML(testStringer{}.String()),
			tag:  stringerGetter{stringer},
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
			got := MakeHtmlGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeHtmlGetter()\n  got %#v\n want %#v", got, tt.want)
			}
			if txt := got.JawsGetHtml(nil); txt != tt.out {
				t.Errorf("MakeHtmlGetter().JawsGetHtml() = %v, want %v", txt, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("MakeHtmlGetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
