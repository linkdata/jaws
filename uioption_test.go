package jaws

import (
	"strings"
	"testing"
)

func TestUiOption(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	nba := NewNamedBoolArray()
	nb := NewNamedBool(nba, "escape\"me", "<unescaped>", true)

	ui := UiOption{nb}
	elem := rq.NewElement(ui)
	var sb strings.Builder
	if err := ui.JawsRender(elem, &sb, []any{"hidden"}); err != nil {
		t.Fatal(err)
	}
	wantHTML := "<option id=\"Jid.1\" hidden value=\"escape&#34;me\" selected><unescaped></option>"
	if gotHTML := sb.String(); gotHTML != wantHTML {
		t.Errorf("got %q, want %q", gotHTML, wantHTML)
	}

	nb.Set(false)
	rq.Dirty(nb)
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "RAttr\tJid.1\t\"selected\"\n" {
			t.Errorf("%q", s)
		}
	}

	nb.Set(true)
	rq.Dirty(nb)
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "SAttr\tJid.1\t\"selected\\n\"\n" {
			t.Errorf("%q", s)
		}
	}
}
