package core

import (
	"html/template"
	"reflect"
	"testing"
)

func TestHTMLGetterFunc(t *testing.T) {
	tt := &testSelfTagger{}
	hg := HTMLGetterFunc(func(e *Element) template.HTML {
		return "foo"
	}, tt)
	if s := hg.JawsGetHTML(nil); s != "foo" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, hg); !reflect.DeepEqual(tags, []any{tt}) {
		t.Error(tags)
	}
}
