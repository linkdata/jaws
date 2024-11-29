package jaws

import (
	"testing"
)

func TestStringGetterFunc(t *testing.T) {
	sg := StringGetterFunc(func(e *Element) string {
		return "foo"
	})
	if s := sg.JawsGetString(nil); s != "foo" {
		t.Error(s)
	}
}
