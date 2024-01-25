package jaws

import (
	"testing"
)

func TestString_StringSetter(t *testing.T) {
	var s String
	s.JawsSetString(nil, "foo")
	if s.JawsGetString(nil) != "foo" {
		t.Fail()
	}
	if s.String() != s.Get() {
		t.Fail()
	}
}

func TestString_HtmlGetter(t *testing.T) {
	var s String
	s.Set("<foo>")
	if v := s.JawsGetHtml(nil); v != "&lt;foo&gt;" {
		t.Errorf("%q", v)
	}
}
