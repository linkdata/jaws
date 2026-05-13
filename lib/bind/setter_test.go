package bind

import (
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

const testStringGetterText = "<span>"

type testGetterString struct{}

func (testGetterString) JawsGet(elem *jaws.Element) string {
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
	if gotTag := setter1.(tag.TagGetter).JawsGetTag(nil); gotTag != tsg {
		t.Error(gotTag)
	}

	setter2 := MakeSetter[string]("quux")
	if err := setter2.JawsSet(nil, "foo"); err != ErrValueNotSettable {
		t.Error(err)
	}
	if s := setter2.JawsGet(nil); s != "quux" {
		t.Error(s)
	}
	if gotTag := setter2.(tag.TagGetter).JawsGetTag(nil); gotTag != nil {
		t.Error(gotTag)
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

func TestMakeSetter_SetterPassThrough(t *testing.T) {
	s := MakeSetter[string]("x")
	if got := s.JawsGet(nil); got != "x" {
		t.Fatalf("unexpected setter getter value %q", got)
	}
	if err := s.JawsSet(nil, "x"); err != ErrValueNotSettable {
		t.Fatalf("unexpected err: %v", err)
	}

	s2 := MakeSetter[string](Setter[string](setterStatic[string]{v: "z"}))
	if got := s2.JawsGet(nil); got != "z" {
		t.Fatalf("unexpected passthrough setter value %q", got)
	}
}
