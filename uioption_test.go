package jaws

import (
	"strings"
	"testing"
	"time"
)

func TestUiOption(t *testing.T) {
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	nba := NewNamedBoolArray()
	nb := NewNamedBool(nba, "escape\"me", "<unescaped>", true)

	ui := UiOption{nb}
	elem := rq.NewElement(ui)
	var sb strings.Builder
	ui.JawsRender(elem, &sb, []any{"hidden"})
	wantHtml := "<option id=\"Jid.1\" hidden value=\"escape&#34;me\" selected><unescaped></option>"
	if gotHtml := sb.String(); gotHtml != wantHtml {
		t.Errorf("got %q, want %q", gotHtml, wantHtml)
	}

	nb.Set(false)
	rq.Dirty(nb)
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-rq.outCh:
		if s != "RAttr\tJid.1\t\"selected\"\n" {
			t.Errorf("%q", s)
		}
	}

	nb.Set(true)
	rq.Dirty(nb)
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-rq.outCh:
		if s != "SAttr\tJid.1\t\"selected\\n\"\n" {
			t.Errorf("%q", s)
		}
	}
}
