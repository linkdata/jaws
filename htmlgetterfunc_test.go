package jaws

import (
	"html/template"
	"reflect"
	"testing"
)

func TestHtmlGetterFunc(t *testing.T) {
	tt := &testSelfTagger{}
	hg := HtmlGetterFunc(func(e *Element) template.HTML {
		return "foo"
	}, tt)
	if s := hg.JawsGetHtml(nil); s != "foo" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, hg); !reflect.DeepEqual(tags, []any{tt}) {
		t.Error(tags)
	}
}
