package jaws

import (
	"reflect"
	"testing"
)

func TestStringGetterFunc(t *testing.T) {
	tt := &testSelfTagger{}
	sg := StringGetterFunc(func(e *Element) string {
		return "foo"
	}, tt)
	if s := sg.JawsGetString(nil); s != "foo" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, sg); !reflect.DeepEqual(tags, []any{tt}) {
		t.Error(tags)
	}
}
