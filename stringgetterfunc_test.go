package jaws

import (
	"reflect"
	"testing"
)

func TestStringGetterFunc(t *testing.T) {
	tt := &testSelfTagger{}
	sg := StringGetterFunc(func(e ElementIf) string {
		return "foo"
	}, tt)
	if s := sg.JawsGet(nil); s != "foo" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, sg); !reflect.DeepEqual(tags, []any{tt}) {
		t.Error(tags)
	}
}
