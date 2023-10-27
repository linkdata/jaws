package jaws

import (
	"testing"

	"github.com/linkdata/jaws/what"
)

type testJawsClick struct {
	clickCh chan string
	*testSetter[string]
}

func (tje *testJawsClick) JawsClick(e *Element, name string) (err error) {
	if err = tje.err; err == nil {
		tje.clickCh <- name
	}
	return
}

var _ ClickHandler = (*testJawsClick)(nil)

func Test_clickHandlerWapper_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	tjc := &testJawsClick{
		clickCh:    make(chan string),
		testSetter: newTestSetter(""),
	}

	want := `<div id="Jid.1">inner</div>`
	rq.Div("inner", tjc)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.Div() = %q, want %q", got, want)
	}

	rq.inCh <- wsMsg{Data: "text", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		t.Errorf("%q", s)
	default:
	}

	rq.inCh <- wsMsg{Data: "adam", Jid: 1, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case name := <-tjc.clickCh:
		if name != "adam" {
			t.Error(name)
		}
	}
}
