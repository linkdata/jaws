package ui

import (
	"errors"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/jtag"
)

type testClickableStringer struct {
	s string
}

func (s testClickableStringer) String() string {
	return s.s
}

func TestClickable_ForwardsClickAndGetterBehavior(t *testing.T) {
	_, rq := newCoreRequest(t)
	elem := rq.NewElement(NewSpan(testHTMLGetter("x")))
	inner := testClickableStringer{s: "<b>x</b>"}
	wantErr := errors.New("boom")

	var gotElem *jaws.Element
	var gotClick jaws.Click

	handler := Clickable(inner, func(elem *jaws.Element, click jaws.Click) (err error) {
		gotElem = elem
		gotClick = click
		return wantErr
	})

	htmlGetter, ok := handler.(bind.HTMLGetter)
	if !ok {
		t.Fatalf("%T does not implement bind.HTMLGetter", handler)
	}
	if got, want := string(htmlGetter.JawsGetHTML(elem)), "&lt;b&gt;x&lt;/b&gt;"; got != want {
		t.Fatalf("want %q got %q", want, got)
	}

	tagGetter, ok := handler.(jtag.TagGetter)
	if !ok {
		t.Fatalf("%T does not implement jtag.TagGetter", handler)
	}
	if got, want := tagGetter.JawsGetTag(rq), any(inner); got != want {
		t.Fatalf("want tag %#v got %#v", want, got)
	}

	if err := handler.JawsClick(elem, jaws.Click{Name: "save"}); !errors.Is(err, wantErr) {
		t.Fatalf("want %v got %v", wantErr, err)
	}
	if gotElem != elem {
		t.Fatalf("expected callback element %p got %p", elem, gotElem)
	}
	if gotClick.Name != "save" {
		t.Fatalf("want name %q got %q", "save", gotClick.Name)
	}
}

func TestClickable_TagIsNilWhenInnerHTMLHasNoTag(t *testing.T) {
	handler := Clickable("plain", func(*jaws.Element, jaws.Click) error { return nil })
	tagGetter, ok := handler.(jtag.TagGetter)
	if !ok {
		t.Fatalf("%T does not implement jtag.TagGetter", handler)
	}
	if tag := tagGetter.JawsGetTag(nil); tag != nil {
		t.Fatalf("expected nil tag, got %#v", tag)
	}
}
