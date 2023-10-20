package jaws

import (
	"fmt"
	"html/template"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

type testJawsEvent struct {
	eventCalled chan string
}

func (tje *testJawsEvent) JawsClick(e *Element, name string) (err error) {
	tje.eventCalled <- fmt.Sprintf("JawsClick: %q", name)
	return
}

func (tje *testJawsEvent) JawsEvent(e *Element, wht what.What, val string) (err error) {
	tje.eventCalled <- fmt.Sprintf("JawsEvent: %s %q", wht, val)
	return
}

func (tje *testJawsEvent) JawsGetTag(*Request) (tag any) {
	return
}

var _ ClickHandler = (*testJawsEvent)(nil)
var _ EventHandler = (*testJawsEvent)(nil)
var _ TagGetter = (*testJawsEvent)(nil)

func TestUiHtml_JawsEvent(t *testing.T) {
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	eventCalled := make(chan string)
	defer close(eventCalled)
	tje := &testJawsEvent{eventCalled: eventCalled}

	id := rq.Register(Tag("zomg"), tje, "attr1", []string{"attr2"}, template.HTML("attr3"), []template.HTML{"attr4"})

	rq.inCh <- wsMsg{Data: "text", Jid: id, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-tje.eventCalled:
		if s != "JawsEvent: Input \"text\"" {
			t.Error(s)
		}
	}

	rq.inCh <- wsMsg{Data: "name", Jid: id, What: what.Click}
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-eventCalled:
		if s != "JawsClick: \"name\"" {
			t.Error(s)
		}
	}

	id2 := rq.Register(tje)
	rq.inCh <- wsMsg{Data: "text", Jid: id2, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-tje.eventCalled:
		if s != "JawsEvent: Input \"text\"" {
			t.Error(s)
		}
	}

	rq.inCh <- wsMsg{Data: "name", Jid: id2, What: what.Click}
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-eventCalled:
		if s != "JawsClick: \"name\"" {
			t.Error(s)
		}
	}
}
