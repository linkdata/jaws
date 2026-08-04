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
	if got := mustExpand(t, hg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}

// TestHTMLGetterFunc_SnapshotsTagSlots is the regression for issue #217: the
// constructor took the caller's variadic slice header, so reusing that slice after
// construction retagged a live getter. Only the top-level slots are copied, which is
// what this asserts.
func TestHTMLGetterFunc_SnapshotsTagSlots(t *testing.T) {
	tags := []any{tag.Tag("original")}
	hg := HTMLGetterFunc(func(*jaws.Element) template.HTML { return "x" }, tags...)
	tags[0] = tag.Tag("reused")

	if got := mustExpand(t, hg); !reflect.DeepEqual(got, []any{tag.Tag("original")}) {
		t.Fatalf("getter tags = %#v, want the construction-time snapshot", got)
	}
}
