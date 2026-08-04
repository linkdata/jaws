package bind

import (
	"reflect"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

func TestStringGetterFunc(t *testing.T) {
	tt := &selfTagger{}
	sg := StringGetterFunc(func(elem *jaws.Element) string {
		return "foo"
	}, tt)
	if s := sg.JawsGet(nil); s != "foo" {
		t.Error(s)
	}
	if got := mustExpand(t, sg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}

// TestStringGetterFunc_SnapshotsTagSlots is the regression for issue #217; see
// TestHTMLGetterFunc_SnapshotsTagSlots for the shape of the bug.
func TestStringGetterFunc_SnapshotsTagSlots(t *testing.T) {
	tags := []any{tag.Tag("original")}
	sg := StringGetterFunc(func(*jaws.Element) string { return "x" }, tags...)
	tags[0] = tag.Tag("reused")

	if got := mustExpand(t, sg); !reflect.DeepEqual(got, []any{tag.Tag("original")}) {
		t.Fatalf("getter tags = %#v, want the construction-time snapshot", got)
	}
}
