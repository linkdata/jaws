package ui

import (
	"html/template"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/named"
)

func TestRequest_RadioGroup(t *testing.T) {
	_, rq := newCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	nba := named.NewBoolArray(false)
	nba.Add("1", "one")
	rel := rw.RadioGroup(nba)

	gotHTML := string(rel[0].Radio("radioattr"))
	if gotHTML != `<input id="Jid.1" type="radio" name="Jid.1" radioattr>` {
		t.Errorf("unexpected radio HTML %q", gotHTML)
	}

	wantHTML := "<label id=\"Jid.2\" for=\"Jid.1\" labelattr>one</label>"
	gotHTML = string(rel[0].Label("labelattr"))
	if gotHTML != wantHTML {
		t.Errorf("got %q, want %q", gotHTML, wantHTML)
	}
}

// TestRequest_RadioGroup_DoesNotMutateCallerParams verifies Radio and Label do
// not write their generated attribute into the caller's variadic backing array
// when it has spare capacity (cap > len).
func TestRequest_RadioGroup_DoesNotMutateCallerParams(t *testing.T) {
	_, rq := newCoreRequest(t)
	rw := RequestWriter{Request: rq, Writer: &strings.Builder{}}

	nba := named.NewBoolArray(false)
	nba.Add("1", "one")
	rel := rw.RadioGroup(nba)

	// len 1, spare capacity: a buggy append(params, internalAttr) would write the
	// framework attribute into index 1, past the caller's length.
	radioParams := make([]any, 1, 4)
	radioParams[0] = template.HTMLAttr(`class="x"`)
	_ = rel[0].Radio(radioParams...)
	if full := radioParams[:cap(radioParams)]; full[1] != nil {
		t.Fatalf("Radio mutated caller params backing array: %v", full)
	}

	labelParams := make([]any, 1, 4)
	labelParams[0] = template.HTMLAttr(`class="y"`)
	_ = rel[0].Label(labelParams...)
	if full := labelParams[:cap(labelParams)]; full[1] != nil {
		t.Fatalf("Label mutated caller params backing array: %v", full)
	}
}

// TestRequest_RadioGroup_GeneratedAttrWins verifies the framework-controlled
// name= / for= attribute takes precedence over a caller-supplied duplicate: it
// is emitted first, and the HTML parser keeps the first of duplicate attributes.
func TestRequest_RadioGroup_GeneratedAttrWins(t *testing.T) {
	_, rq := newCoreRequest(t)
	rw := RequestWriter{Request: rq, Writer: &strings.Builder{}}

	nba := named.NewBoolArray(false)
	nba.Add("1", "one")
	rel := rw.RadioGroup(nba)

	got := string(rel[0].Radio(template.HTMLAttr(`name="caller"`)))
	if want := `<input id="Jid.1" type="radio" name="Jid.1" name="caller">`; got != want {
		t.Errorf("Radio: got %q, want %q", got, want)
	}

	gotLabel := string(rel[0].Label(template.HTMLAttr(`for="caller"`)))
	if want := `<label id="Jid.2" for="Jid.1" for="caller">one</label>`; gotLabel != want {
		t.Errorf("Label: got %q, want %q", gotLabel, want)
	}
}

// TestRequest_RadioGroup_LazyCreation verifies options that are never rendered
// register no Elements on the Request, and that rendering an option creates its
// radio (Jid before the label) and label exactly once.
func TestRequest_RadioGroup_LazyCreation(t *testing.T) {
	_, rq := newCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	nba := named.NewBoolArray(false)
	nba.Add("1", "one")
	nba.Add("2", "two")
	rel := rw.RadioGroup(nba)

	// Nothing rendered yet: no Elements should exist.
	if rq.GetElementByJid(1) != nil {
		t.Fatal("RadioGroup created Elements before any option was rendered")
	}

	// Render only the first option.
	_ = rel[0].Radio()
	_ = rel[0].Label()
	if rq.GetElementByJid(1) == nil || rq.GetElementByJid(2) == nil {
		t.Fatal("rendering an option must create its radio and label Elements")
	}

	// The second option was never rendered; it must not have registered Elements.
	if rq.GetElementByJid(3) != nil {
		t.Fatal("an unrendered option must not register any Element")
	}
}

// TestRequest_RadioGroup_LabelBeforeRadio verifies the radioElem() invariant: the
// radio Element is created (and so receives the lower Jid) before the label even
// when a template renders Label before Radio, so the label's for= attribute always
// references the radio's Jid. The other RadioGroup tests render Radio first, so
// this Label-first path would otherwise be uncovered.
func TestRequest_RadioGroup_LabelBeforeRadio(t *testing.T) {
	_, rq := newCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	nba := named.NewBoolArray(false)
	nba.Add("1", "one")
	rel := rw.RadioGroup(nba)

	// Render Label before Radio. The radio must already exist at the lower Jid so
	// the label gets the next Jid and points its for= at the radio.
	wantLabel := "<label id=\"Jid.2\" for=\"Jid.1\">one</label>"
	if gotLabel := string(rel[0].Label()); gotLabel != wantLabel {
		t.Errorf("Label-first: got %q, want %q", gotLabel, wantLabel)
	}
	if rq.GetElementByJid(1) == nil {
		t.Fatal("radio Element (Jid.1) must be created when Label is rendered first")
	}

	// Rendering Radio afterwards reuses the same Jid.1 element.
	if gotRadio := string(rel[0].Radio()); gotRadio != `<input id="Jid.1" type="radio" name="Jid.1">` {
		t.Errorf("radio should render at Jid.1, got %q", gotRadio)
	}
}

func TestRequest_RadioGroup_NameUsesFirstRadioJid(t *testing.T) {
	_, rq := newCoreRequest(t)
	rw := RequestWriter{Request: rq, Writer: &strings.Builder{}}

	nba := named.NewBoolArray(false)
	nba.Add("1", "one")
	nba.Add("2", "two")
	rel := rw.RadioGroup(nba)

	// Rendering the second option first makes its Jid the group name. Every later
	// option in the same group must reuse that request-scoped identity.
	if got := string(rel[1].Radio()); got != `<input id="Jid.1" type="radio" name="Jid.1">` {
		t.Fatalf("first rendered option = %q", got)
	}
	if got := string(rel[0].Radio()); got != `<input id="Jid.2" type="radio" name="Jid.1">` {
		t.Fatalf("second rendered option = %q", got)
	}

	other := rw.RadioGroup(nba)
	if got := string(other[0].Radio()); got != `<input id="Jid.3" type="radio" name="Jid.3">` {
		t.Fatalf("separate group = %q", got)
	}
}
