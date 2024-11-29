package jaws

import (
	"html/template"
	"testing"
)

func TestHtmlGetterFunc(t *testing.T) {
	hg := HtmlGetterFunc(func(e *Element) template.HTML {
		return "foo"
	})
	if s := hg.JawsGetHtml(nil); s != "foo" {
		t.Error(s)
	}
}
