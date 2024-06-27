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

	val := "text'1"
	ts := newTestSetter(val)
	th.NoErr(rq.JsString(ts, "varname"))
	wantHtml := "<div id=\"Jid.1\" data-jawsdata='\"text\\u00271\"' data-jawsname=\"varname\" hidden></div>"
	if gotHtml := rq.BodyString(); gotHtml != wantHtml {
		t.Errorf("Request.JsString() = %q, want %q", gotHtml, wantHtml)
	}

	ts.Set("text'2")
	rq.Dirty(ts)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Set\tJid.1\t\"text'2\"\n" {
			t.Error(strconv.Quote(s))
		}
	}
}
