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
	*testSetter[template.HTML]
}

func (tje *testJawsEvent) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if err = tje.err; err == nil {
		tje.eventCalled <- fmt.Sprintf("%s %q", wht, val)
	}
	return
}

var _ EventHandler = (*testJawsEvent)(nil)

func TestRequest_Register(t *testing.T) {
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	tje := &testJawsEvent{
		eventCalled: make(chan string),
		testSetter:  newTestSetter(template.HTML("meh")),
	}

	id := rq.Register(Tag("zomg"), tje, "attr1", []string{"attr2"}, template.HTML("attr3"), []template.HTML{"attr4"})

	rq.inCh <- wsMsg{Data: "text", Jid: id, What: what.Input}
	select {
	case <-tmr.C:
		t.Error("timeout")
	case s := <-tje.eventCalled:
		if s != "Input \"text\"" {
			t.Error(s)
		}
	}
}
