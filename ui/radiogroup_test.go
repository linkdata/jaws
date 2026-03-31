package ui

import (
	"strings"
	"testing"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/named"
	"github.com/linkdata/jaws/internal/testutil"
)

func TestRequest_RadioGroup(t *testing.T) {
	core.NextJid = 0
	_, rq := testutil.NewCoreRequest(t)
	var sb strings.Builder
	rw := RequestWriter{Request: rq, Writer: &sb}

	nba := named.NewNamedBoolArray(false)
	nba.Add("1", "one")
	rel := rw.RadioGroup(nba)

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
