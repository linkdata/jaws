package jaws

import (
	"testing"
)

const testStringGetterText = "<span>"

type testGetterString struct{}

func (testGetterString) JawsGet(*Element) string {
	return testStringGetterText
}

func Test_makeSetter(t *testing.T) {
	tsg := testGetterString{}
	setter1 := MakeSetter[string](tsg)
	if err := setter1.JawsSet(nil, "foo"); err != ErrValueNotSettable {
		t.Error(err)
	}
	if s := setter1.JawsGet(nil); s != testStringGetterText {
		t.Error(s)
	}
	if tag := setter1.(TagGetter).JawsGetTag(nil); tag != tsg {
		t.Error(tag)
	}

	setter2 := MakeSetter[string]("quux")
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
	setter2 := MakeSetter[string](123)
	t.Error(setter2)
}
