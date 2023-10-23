package jaws

import (
	"testing"
)

func TestRequest_RadioGroup(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	nba := NewNamedBoolArray()
	nba.Add("1", "one")
	rel := rq.RadioGroup(nba)

	wantHtml := "<input id=\"Jid.2\" type=\"radio\" radioattr name=\"jaws.1\">"
	gotHtml := string(rel[0].Radio("radioattr"))
	if gotHtml != wantHtml {
		t.Errorf("got %q, want %q", gotHtml, wantHtml)
	}

	wantHtml = "<label id=\"Jid.3\" labelattr for=\"Jid.2\">one</label>"
	gotHtml = string(rel[0].Label("labelattr"))
	if gotHtml != wantHtml {
		t.Errorf("got %q, want %q", gotHtml, wantHtml)
	}
}
