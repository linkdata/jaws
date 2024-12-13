package jaws

import (
	"testing"
)

type testStringGetter struct{}

func (testStringGetter) JawsGet(*Element) string {
	return "static"
}

func Test_makeSetter(t *testing.T) {
	tsg := testStringGetter{}
	setter1 := makeSetter[string](tsg)
	if err := setter1.JawsSet(nil, "foo"); err != ErrValueNotSettable {
		t.Error(err)
	}
	if s := setter1.JawsGet(nil); s != "static" {
		t.Error(s)
	}
	if tag := setter1.(TagGetter).JawsGetTag(nil); tag != tsg {
		t.Error(tag)
	}

	setter2 := makeSetter[string]("quux")
	if err := setter2.JawsSet(nil, "foo"); err != ErrValueNotSettable {
		t.Error(err)
	}
	if s := setter2.JawsGet(nil); s != "quux" {
		t.Error(s)
	}
	if tag := setter2.(TagGetter).JawsGetTag(nil); tag != nil {
		t.Error(tag)
	}
}

func Test_makeSetter_panic(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Fail()
		}
	}()
	setter2 := makeSetter[string](123)
	t.Error(setter2)
}
