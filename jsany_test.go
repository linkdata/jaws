package jaws

import (
	"strconv"
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestJsAny_JawsRender(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	var val any
	ts := newTestSetter(val)
	th.NoErr(rq.JsAny(ts, "varname"))
	wantHtml := "<div id=\"Jid.1\" data-jawsname=\"varname\" hidden></div>"
	if gotHtml := rq.BodyString(); gotHtml != wantHtml {
		t.Errorf("Request.JsAny() = %q, want %q", gotHtml, wantHtml)
	}

	ts.Set(1.3)
	rq.Dirty(ts)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Set\tJid.1\t1.3\n" {
			t.Error(strconv.Quote(s))
		}
	}
}

func TestJsAny_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)

	val := float64(1.2)
	ts := newTestSetter(any(val))
	ui := NewJsAny(ts, "varname")
	th.NoErr(rq.UI(ui))

	th.Equal(ui.JawsGetTag(rq.Request), ts)

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "1.3"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-ts.setCalled:
	}

	th.Equal(ts.Get(), 1.3)

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "1.3"}:
	}
}
