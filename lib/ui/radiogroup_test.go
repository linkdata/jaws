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
