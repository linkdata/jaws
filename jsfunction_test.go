package jaws

import (
	"strconv"
	"testing"

	"github.com/linkdata/jaws/what"
)

func TestJsFunction_JawsRender(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	param := any("meh")
	tsparam := newTestSetter(param)
	result := any("foo")
	tsresult := newTestSetter(result)
	ui := NewJsFunction(tsparam, tsresult, "fnname")
	th.NoErr(rq.UI(ui))
	wantHtml := "<div id=\"Jid.1\" data-jawsname=\"fnname\" hidden></div>"
	if gotHtml := rq.BodyString(); gotHtml != wantHtml {
		t.Errorf("Request.JsFunction() = %q, want %q", gotHtml, wantHtml)
	}

	tsparam.Set(1.3)
	rq.Dirty(tsparam)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if s != "Call\tJid.1\t1.3\n" {
			t.Error(strconv.Quote(s))
		}
	}
}

func TestJsFunction_PanicsIfParamNil(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	defer func() {
		if x := recover(); x == nil {
			is.Fail()
		}
	}()
	NewJsFunction(nil, nil, "fnname")
	is.Fail()
}

func TestJsFunction_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	param := any("foo")
	tsparam := newTestSetter(param)
	result := any(float64(1.2))
	tsresult := newTestSetter(result)
	th.NoErr(rq.JsFunction(tsparam, tsresult, "fnname"))

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "1.3"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-tsresult.setCalled:
	}

	th.Equal(tsresult.Get(), 1.3)

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "1.3"}:
	}
}
