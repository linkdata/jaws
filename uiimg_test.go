package jaws

import (
	"strconv"
	"testing"
	"time"
)

func TestRequest_Img(t *testing.T) {
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	ts := newTestSetter("\"quoted.png\"")

	wantHtml := "<img id=\"Jid.1\" hidden src=\"quoted.png\">"
	rq.Img(ts, "hidden")
	if gotHtml := rq.BodyString(); gotHtml != wantHtml {
		t.Errorf("Request.Img() = %q, want %q", gotHtml, wantHtml)
	}

	tmr := time.NewTimer(testTimeout)
	ts.Set("unquoted.jpg")
	rq.Dirty(ts)

	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-rq.outCh:
		if s != "SAttr\tJid.1\t\"src\\n\\\"unquoted.jpg\\\"\"\n" {
			t.Error(strconv.Quote(s))
		}
	}
}
