package jaws

import (
	"fmt"
	"html/template"
	"io"
	"testing"

	"github.com/linkdata/jaws/what"
)

type testJawsEvent struct {
	msgCh    chan string
	tag      any
	clickerr error
	eventerr error
}

func (t *testJawsEvent) JawsClick(e *Element, name string) (err error) {
	if err = t.clickerr; err == nil {
		t.msgCh <- fmt.Sprintf("JawsClick: %q", name)
	}
	return
}

func (t *testJawsEvent) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if err = t.eventerr; err == nil {
		t.msgCh <- fmt.Sprintf("JawsEvent: %s %q", wht, val)
	} else {
		t.msgCh <- err.Error()
	}
	return
}

func (t *testJawsEvent) JawsGetTag(*Request) (tag any) {
	return t.tag
}

func (t *testJawsEvent) JawsRender(e *Element, w io.Writer, params []any) error {
	w.Write([]byte(fmt.Sprint(params)))
	t.msgCh <- fmt.Sprintf("JawsRender(%d)", e.jid)
	return nil
}

func (t *testJawsEvent) JawsUpdate(e *Element) {
	t.msgCh <- fmt.Sprintf("JawsUpdate(%d)", e.jid)
}

var _ ClickHandler = (*testJawsEvent)(nil)
var _ EventHandler = (*testJawsEvent)(nil)
var _ TagGetter = (*testJawsEvent)(nil)
var _ UI = (*testJawsEvent)(nil)

func Test_JawsEvent_ClickUnhandled(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}
	zomgItem := &testUi{}
	id := rq.Register(zomgItem, je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	je.clickerr = ErrEventUnhandled
	rq.inCh <- wsMsg{Data: "name", Jid: id, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != "JawsEvent: Click \"name\"" {
			t.Error(s)
		}
	}
}

func Test_JawsEvent_AllUnhandled(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}
	zomgItem := &testUi{}
	id := rq.Register(zomgItem, je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	je.clickerr = ErrEventUnhandled
	je.eventerr = ErrEventUnhandled
	rq.inCh <- wsMsg{Data: "name", Jid: id, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != ErrEventUnhandled.Error() {
			t.Error(s)
		}
	}
}
