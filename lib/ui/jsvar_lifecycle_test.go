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
