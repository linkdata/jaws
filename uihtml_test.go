package jaws

import (
	"fmt"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/linkdata/jaws/what"
)

type testJawsEvent struct {
	UiHtml
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

func TestUiHtml_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}

	id := rq.Register(Tag("zomg"), je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	rq.inCh <- wsMsg{Data: "text", Jid: id, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-je.msgCh:
		if s != "JawsEvent: Input \"text\"" {
			t.Error(s)
		}
	}

	rq.inCh <- wsMsg{Data: "name", Jid: id, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != "JawsClick: \"name\"" {
			t.Error(s)
		}
	}

	je.tag = je
	id2 := rq.Register(je)
	th.Equal(id2, Jid(2))

	rq.inCh <- wsMsg{Data: "text2", Jid: id2, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-je.msgCh:
		if s != "JawsEvent: Input \"text2\"" {
			t.Error(s)
		}
	}

	// nothing should be marked dirty,
	// but if it is, this ensures the
	// test fails reliably
	rq.jw.distributeDirt()

	rq.inCh <- wsMsg{Data: "name2", Jid: id2, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != "JawsClick: \"name2\"" {
			t.Error(s)
		}
	}

	rq.Dirty(je)
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != "JawsUpdate(2)" {
			t.Error(s)
		}
	}

	elem := rq.getElementByJid(id2)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, []any{"attr"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != "JawsRender(2)" {
			t.Error(s)
		}
	}
	if x := sb.String(); x != "[attr]" {
		t.Error(x)
	}
}

func Test_JawsEvent_ClickUnhandled(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}

	id := rq.Register(Tag("zomg"), je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

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

	id := rq.Register(Tag("zomg"), je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

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
