package jaws

import (
	"encoding/json"
	"testing"
)

func TestString_StringSetter(t *testing.T) {
	var s String
	if err := s.JawsSetString(nil, "foo"); err != nil {
		t.Error(err)
	}
	if s.JawsGetString(nil) != "foo" {
		t.Fail()
	}
	if s.String() != s.Get() {
		t.Fail()
	}
	if err := s.JawsSetString(nil, "foo"); err != ErrValueUnchanged {
		t.Error(err)
	}
}

func TestString_HtmlGetter(t *testing.T) {
	s := String{Value: "<foo>"}
	if v := s.JawsGetHtml(nil); v != "&lt;foo&gt;" {
		t.Errorf("%q", v)
	}
}

func TestString_Marshalling(t *testing.T) {
	var s, s2 String
	s.Set("foo")
	b, err := json.Marshal(&s)
	if err != nil {
		t.Error(err)
	} else {
		if string(b) != "\"foo\"" {
			t.Error(string(b))
		}
	}
	err = json.Unmarshal(b, &s2)
	if err != nil {
		t.Error(err)
	} else {
		if s2.Value != "foo" {
			t.Error(s2.Value)
		}
	}
}
