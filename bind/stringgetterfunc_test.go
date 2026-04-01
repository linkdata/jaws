package bind

import (
	"reflect"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstags"
)

func TestStringGetterFunc(t *testing.T) {
	tt := &selfTagger{}
	sg := StringGetterFunc(func(e *jaws.Element) string {
		return "foo"
	}, tt)
	if s := sg.JawsGet(nil); s != "foo" {
		t.Error(s)
	}
	if got := jawstags.MustTagExpand(nil, sg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}
