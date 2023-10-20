package jaws

import (
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

func Test_clickHandlerWapper_JawsEvent(t *testing.T) {
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter(false)

	want := `<div id="Jid.1">inner</div>`
	if got := string(rq.Div("inner", ts)); got != want {
		t.Errorf("Request.Div() = %q, want %q", got, want)
	}

	rq.inCh <- wsMsg{Data: "text", Jid: 1, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-rq.outCh:
		t.Errorf("%q", s)
	default:
	}

	rq.inCh <- wsMsg{Data: "adam", Jid: 1, What: what.Click}
	select {
	case <-tmr.C:
		t.Error("timeout")
	case name := <-ts.clickCh:
		if name != "adam" {
			t.Error(name)
		}
	}
}
