package ui

import (
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
	if !strings.HasPrefix(gotHTML, "<input id=\"Jid.1\" type=\"radio\" radioattr name=\"jaws.") || !strings.HasSuffix(gotHTML, "\">") {
		t.Errorf("unexpected radio HTML %q", gotHTML)
	}

	wantHTML := "<label id=\"Jid.2\" labelattr for=\"Jid.1\">one</label>"
	gotHTML = string(rel[0].Label("labelattr"))
	if gotHTML != wantHTML {
		t.Errorf("got %q, want %q", gotHTML, wantHTML)
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
