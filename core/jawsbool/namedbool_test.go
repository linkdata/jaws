package jawsbool

import (
	"html/template"
	"testing"
)

func TestNamedBool(t *testing.T) {
	nba := NewNamedBoolArray(false)
	nba.Add("1", "one")
	nb := nba.data[0]

	_, rq := newCoreRequest(t)
	e := rq.NewElement(noopUI{})

	if nb.Array() != nba {
		t.Fatalf("array mismatch: got %p want %p", nb.Array(), nba)
	}
	if nb.Name() != "1" {
		t.Fatalf("name mismatch: got %q want %q", nb.Name(), "1")
	}
	if nb.HTML() != template.HTML("one") {
		t.Fatalf("html mismatch: got %q want %q", nb.HTML(), template.HTML("one"))
	}

	if got := nb.JawsGetHTML(nil); got != nb.HTML() {
		t.Fatalf("JawsGetHTML mismatch: got %q want %q", got, nb.HTML())
	}

	if err := nb.JawsSet(e, true); err != nil {
		t.Fatal(err)
	}
	if !nb.Checked() {
		t.Fatal("expected checked true")
	}
	if got := nb.JawsGet(nil); got != nb.Checked() {
		t.Fatalf("JawsGet mismatch: got %v want %v", got, nb.Checked())
	}
	if err := nb.JawsSet(e, true); err != ErrValueUnchanged {
		t.Fatalf("expected ErrValueUnchanged, got %v", err)
	}
}
