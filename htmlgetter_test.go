package jaws

import (
	"html"
	"html/template"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

var _ HtmlGetter = (*testSetter[template.HTML])(nil)

func Test_makeHtmlGetter_panic(t *testing.T) {
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
	makeHtmlGetter(uint32(42))
}

func Test_makeHtmlGetter(t *testing.T) {
	untypedText := "<span>"
	typedText := template.HTML(untypedText)
	escapedTypedText := template.HTML(html.EscapeString(untypedText))
	var avUntyped, avTyped atomic.Value
	avUntyped.Store(untypedText)
	avTyped.Store(typedText)

	tests := []struct {
		name string
		v    interface{}
		want HtmlGetter
		out  template.HTML
		tag  interface{}
	}{
		{
			name: "HtmlGetter",
			v:    htmlGetter{typedText},
			want: htmlGetter{typedText},
			out:  typedText,
			tag:  nil,
		},
		{
			name: "StringGetter",
			v:    stringGetter{untypedText},
			want: htmlStringGetter{stringGetter{untypedText}},
			out:  escapedTypedText,
			tag:  stringGetter{untypedText},
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
			name: "*atomic.Value(string)",
			v:    &avUntyped,
			want: atomicGetter{&avUntyped},
			out:  escapedTypedText,
			tag:  &avUntyped,
		},
		{
			name: "*atomic.Value(template.HTML)",
			v:    &avTyped,
			want: atomicGetter{&avTyped},
			out:  typedText,
			tag:  &avTyped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeHtmlGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeHtmlGetter() = %v, want %v", got, tt.want)
			}
			if txt := got.JawsGetHtml(nil); txt != tt.out {
				t.Errorf("makeHtmlGetter().JawsGetHtml() = %v, want %v", txt, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeHtmlGetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
