package jaws

import (
	"testing"
)

func TestStringer(t *testing.T) {
	txt := "text"
	if s := Stringer(&txt).String(); s != "text" {
		t.Error(s)
	}

	num := int(123)
	if s := Stringer(&num).String(); s != "123" {
		t.Error(s)
	}
}
