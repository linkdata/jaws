package jaws

import (
	"strconv"
	"testing"
)

func TestRequest_Img(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter("\"quoted.png\"")

	wantHtml := "<img id=\"Jid.1\" hidden src=\"quoted.png\">"
	rq.Img(ts, "hidden")
	if gotHtml := rq.BodyString(); gotHtml != wantHtml {
		t.Errorf("Request.Img() = %q, want %q", gotHtml, wantHtml)
	}

	ts.Set("unquoted.jpg")
	rq.Dirty(ts)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "SAttr\tJid.1\t\"src\\n\\\"unquoted.jpg\\\"\"\n" {
			t.Error(strconv.Quote(s))
		}
	}
}
