package jaws

import (
	"testing"

	"github.com/linkdata/jaws/core/tags"
)

func Test_makeGetter_panic(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Fail()
		}
	}()
	setter2 := MakeGetter[string](123)
	t.Error(setter2)
}

func Test_makeGetter(t *testing.T) {
	setter := MakeGetter[string]("foo")
	if err := setter.(Setter[string]).JawsSet(nil, "bar"); err != ErrValueNotSettable {
		t.Error(err)
	}
}

func TestMakeGetter_GetterPassThroughAndTag(t *testing.T) {
	g := MakeGetter[string]("x")
	if got := g.JawsGet(nil); got != "x" {
		t.Fatalf("unexpected getter value %q", got)
	}
	if tag := g.(tags.TagGetter).JawsGetTag(nil); tag != nil {
		t.Fatalf("expected nil tag, got %#v", tag)
	}

	g2 := MakeGetter[string](Getter[string](getterStatic[string]{v: "y"}))
	if got := g2.JawsGet(nil); got != "y" {
		t.Fatalf("unexpected passthrough getter value %q", got)
	}
}
