package jaws

import (
	"strconv"
	"testing"
)

func TestJsString_JawsRender(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	val := "text1"
	ts := newTestSetter(val)
	th.NoErr(rq.JsString(ts, "varname"))
	wantHtml := "<div id=\"Jvar.varname\" data-json='\"text1\"' data-jid=\"Jid.1\"></div>"
	if gotHtml := rq.BodyString(); gotHtml != wantHtml {
		t.Errorf("Request.JsString() = %q, want %q", gotHtml, wantHtml)
	}

	ts.Set("text2")
	rq.Dirty(ts)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Set\t\t\"varname\\n\\\"text2\\\"\"\n" {
			t.Error(strconv.Quote(s))
		}
	}
}
