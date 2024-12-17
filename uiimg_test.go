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

	ts := newTestSetter("image.png")

	wantHTML := "<img id=\"Jid.1\" hidden src=\"image.png\">"
	rq.Img(ts, "hidden")
	if gotHTML := rq.BodyString(); gotHTML != wantHTML {
		t.Errorf("Request.Img() = %q, want %q", gotHTML, wantHTML)
	}

	ts.Set("image2.jpg")
	rq.Dirty(ts)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "SAttr\tJid.1\t\"src\\nimage2.jpg\"\n" {
			t.Error(strconv.Quote(s))
		}
	}
}
