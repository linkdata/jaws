package jaws

import (
	"strconv"
	"testing"
)

func TestJsNumber_JawsRender(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	val := float64(1.2)
	ts := newTestSetter(val)
	th.NoErr(rq.JsNumber(ts, "varname", "hidden"))
	wantHtml := "<div id=\"Jvar.varname\" data-json='1.2' data-jid=\"Jid.1\" hidden></div>"
	if gotHtml := rq.BodyString(); gotHtml != wantHtml {
		t.Errorf("Request.JsNumber() = %q, want %q", gotHtml, wantHtml)
	}

	ts.Set(1.3)
	rq.Dirty(ts)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Set\t\t\"varname\\n1.3\"\n" {
			t.Error(strconv.Quote(s))
		}
	}
}
