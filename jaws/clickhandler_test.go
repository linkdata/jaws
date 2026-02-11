package jaws

import (
	"html/template"
	"testing"

	"github.com/linkdata/jaws/what"
)

type testJawsClick struct {
	clickCh chan string
	*testSetter[string]
}

func (tjc *testJawsClick) JawsClick(e *Element, name string) (err error) {
	if err = tjc.err; err == nil {
		tjc.clickCh <- name
	}
	return
}

var _ ClickHandler = (*testJawsClick)(nil)

func Test_clickHandlerWapper_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	tjc := &testJawsClick{
		clickCh:    make(chan string),
		testSetter: newTestSetter(""),
	}

	want := `<div id="Jid.1">inner</div>`
	rq.UI(testDivWidget{inner: template.HTML("inner")}, tjc)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.UI(NewDiv()) = %q, want %q", got, want)
	}

	rq.InCh <- WsMsg{Data: "text", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.OutCh:
		t.Errorf("%q", s)
	default:
	}

	rq.InCh <- WsMsg{Data: "adam", Jid: 1, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case name := <-tjc.clickCh:
		if name != "adam" {
			t.Error(name)
		}
	}
}
