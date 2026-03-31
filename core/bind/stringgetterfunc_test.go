package bind

import (
	"reflect"
	"testing"

	"github.com/linkdata/jaws/core/internal/testutil"
	"github.com/linkdata/jaws/core/tags"
)

func TestStringGetterFunc(t *testing.T) {
	tt := &testutil.SelfTagger{}
	sg := StringGetterFunc(func(e *Element) string {
		return "foo"
	}, tt)
	if s := sg.JawsGet(nil); s != "foo" {
		t.Error(s)
	}
	if got := tags.MustTagExpand(nil, sg); !reflect.DeepEqual(got, []any{tt}) {
		t.Error(got)
	}
}
