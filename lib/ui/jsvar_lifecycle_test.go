package ui

import (
	"strings"
	"sync"
	"testing"
)

// TestJsVar_JawsGetTagInitialization covers the one initialization phase the
// tag.TagGetter contract allows: JsVar reports nil until its first render resolves the
// dirty tag from the bound value, and reports that same tag from then on. The nil is
// not a dirty target, which TestJsVar_SetBeforeRenderDoesNotBroadcast covers.
func TestJsVar_JawsGetTagInitialization(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	v := jsVarData{Text: "initial"}
	jsv := NewJsVar(&mu, &v)

	if got := jsv.JawsGetTag(); got != nil {
		t.Fatalf("JawsGetTag() before render = %#v, want nil", got)
	}
	// Repeated pre-render calls stay nil: the transition happens in JawsRender, not on
	// first access.
	if got := jsv.JawsGetTag(); got != nil {
		t.Fatalf("second JawsGetTag() before render = %#v, want nil", got)
	}

	elem := rq.NewElement(jsv)
	var sb strings.Builder
	if err := jsv.JawsRender(elem, &sb, []any{"lifecycle"}); err != nil {
		t.Fatal(err)
	}

	// The bound value is not a tag.TagGetter, so the resolved tag is Ptr itself.
	first := jsv.JawsGetTag()
	if first != any(&v) {
		t.Fatalf("JawsGetTag() after render = %#v, want the bound pointer", first)
	}

	// Past initialization the reported tag is stable, including across a set that
	// broadcasts through it.
	if err := jsv.JawsSetPath(elem, "text", "changed"); err != nil {
		t.Fatal(err)
	}
	if got := jsv.JawsGetTag(); got != first {
		t.Fatalf("JawsGetTag() after JawsSetPath = %#v, want %#v", got, first)
	}
	if got := jsv.JawsGetTag(); got != first {
		t.Fatalf("repeated JawsGetTag() = %#v, want %#v", got, first)
	}
}

// TestJsVar_NotYetRenderedIsNotUsableAsTag pins the consequence of the nil phase that
// JsVar.JawsGetTag documents: expanding a JsVar before its first render yields no keys
// at all, so a widget tagged with it registers under nothing, and dirtying the JsVar
// once it has rendered still does not reach that widget. Rendering the JsVar first is
// what makes it a working tag.
//
// The test tags through jaws.Element.Tag, which expands unconditionally. A JsVar
// arriving via ParseParams or ApplyGetter is also accepted there — it satisfies their
// usable-as-tag check by being a tag.TagGetter — and then reaches the same expansion.
func TestJsVar_NotYetRenderedIsNotUsableAsTag(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.Mutex
	v := jsVarData{Text: "initial"}
	jsv := NewJsVar(&mu, &v)

	// Tag a second widget with the not-yet-rendered JsVar.
	other := rq.NewElement(NewSpan(testHTMLGetter("x")))
	other.Tag(jsv)
	if tags := rq.TagsOf(other); len(tags) != 0 {
		t.Fatalf("a JsVar in its nil phase registered %v, want no keys", tags)
	}

	// Rendering the JsVar resolves its dirty tag, but that does not retroactively
	// register the widget that was tagged during the nil phase.
	elem := rq.NewElement(jsv)
	var sb strings.Builder
	if err := jsv.JawsRender(elem, &sb, []any{"lifecycle"}); err != nil {
		t.Fatal(err)
	}
	if tags := rq.TagsOf(other); len(tags) != 0 {
		t.Fatalf("rendering the JsVar retroactively registered %v, want no keys", tags)
	}

	// Tagging after the render is what works.
	after := rq.NewElement(NewSpan(testHTMLGetter("y")))
	after.Tag(jsv)
	if !after.HasTag(any(&v)) {
		t.Fatalf("after render, tagging with the JsVar registered %v, want the bound pointer", rq.TagsOf(after))
	}
	// HasTag looks up a single registered key and does not expand, so the JsVar that
	// produced the registration is itself not a hit. This is the asymmetry
	// jaws.Request.HasTag documents.
	if after.HasTag(jsv) {
		t.Fatal("HasTag matched an unexpanded tag.TagGetter, want only its expanded key")
	}
}
