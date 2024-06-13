package jaws

import (
	"bytes"
	"fmt"
	"testing"
)

type printlnerImpl struct {
	str string
}

func (x *printlnerImpl) Println(args ...any) {
	x.str += fmt.Sprint(args...)
}

type errorerImpl struct {
	str string
}

func (x *errorerImpl) Error(msg string, args ...any) {
	x.str += msg
}

func TestLogError(t *testing.T) {
	a := printlnerImpl{}
	LogError(&a, "a")
	LogError(&a, "\na\n")
	if a.str != "a\na" {
		t.Errorf("%q", a.str)
	}

	b := errorerImpl{}
	LogError(&b, "b")
	LogError(&b, "\nb\n")
	if b.str != "b\nb" {
		t.Errorf("%q", b.str)
	}

	var c bytes.Buffer
	LogError(&c, "c")
	LogError(&c, "\nc\n")
	if c.String() != "c\n\nc\n" {
		t.Errorf("%q", c.String())
	}
}
