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
	if got := tag.MustTagExpand(nil, sg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}
