package bind

import (
	"html/template"
	"reflect"
	"testing"

	"github.com/linkdata/jaws/core/tags"
)

func TestHTMLGetterFunc(t *testing.T) {
	tt := &testSelfTagger{}
	hg := HTMLGetterFunc(func(e *Element) template.HTML {
		return "foo"
	}, tt)
	if s := hg.JawsGetHTML(nil); s != "foo" {
		t.Error(s)
	}
	if got := tags.MustTagExpand(nil, hg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}
