package jaws

import (
	"testing"
)

func TestStringer(t *testing.T) {
	var x *int
	if s := Stringer(x).String(); s != "<nil>" {
		t.Error(s)
	}

	txt := "text"
	if s := Stringer(&txt).String(); s != "text" {
		t.Error(s)
	}

	num := int(123)
	if s := Stringer(&num).String(); s != "123" {
		t.Error(s)
	}
}
