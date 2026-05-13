package bind

import (
	"html/template"
	"reflect"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

func TestHTMLGetterFunc(t *testing.T) {
	tt := &selfTagger{}
	hg := HTMLGetterFunc(func(elem *jaws.Element) template.HTML {
		return "foo"
	}, tt)
	if s := hg.JawsGetHTML(nil); s != "foo" {
		t.Error(s)
	}
	if got := tag.MustTagExpand(nil, hg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}
