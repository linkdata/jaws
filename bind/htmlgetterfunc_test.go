package bind

import (
	"html/template"
	"reflect"
	"testing"

	"github.com/linkdata/jaws/jawstags"
)

func TestHTMLGetterFunc(t *testing.T) {
	tt := &selfTagger{}
	hg := HTMLGetterFunc(func(e *Element) template.HTML {
		return "foo"
	}, tt)
	if s := hg.JawsGetHTML(nil); s != "foo" {
		t.Error(s)
	}
	if got := jawstags.MustTagExpand(nil, hg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}
