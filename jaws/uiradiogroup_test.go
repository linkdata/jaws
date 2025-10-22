package jaws

import (
	"testing"
)

func TestRequest_RadioGroup(t *testing.T) {
	nextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	nba := NewNamedBoolArray()
	nba.Add("1", "one")
	rel := rq.RadioGroup(nba)

	wantHTML := "<input id=\"Jid.2\" type=\"radio\" radioattr name=\"jaws.1\">"
	gotHTML := string(rel[0].Radio("radioattr"))
	if gotHTML != wantHTML {
		t.Errorf("got %q, want %q", gotHTML, wantHTML)
	}

	wantHTML = "<label id=\"Jid.3\" labelattr for=\"Jid.2\">one</label>"
	gotHTML = string(rel[0].Label("labelattr"))
	if gotHTML != wantHTML {
		t.Errorf("got %q, want %q", gotHTML, wantHTML)
	}
}
