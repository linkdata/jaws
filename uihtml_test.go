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
	msgCh chan string
	tag   any
}

func (tje *testJawsEvent) JawsClick(e *Element, name string) (err error) {
	tje.msgCh <- fmt.Sprintf("JawsClick: %q", name)
	return
}

func (tje *testJawsEvent) JawsEvent(e *Element, wht what.What, val string) (err error) {
	tje.msgCh <- fmt.Sprintf("JawsEvent: %s %q", wht, val)
	return
}

func (tje *testJawsEvent) JawsGetTag(*Request) (tag any) {
	if tje.tag != nil {
		return tje.tag
	}
	return nil
}

func (tje *testJawsEvent) JawsRender(e *Element, w io.Writer, params []any) error {
	w.Write([]byte(fmt.Sprint(params)))
	tje.msgCh <- fmt.Sprintf("JawsRender(%d)", e.jid)
	return nil
}

func (tje *testJawsEvent) JawsUpdate(e *Element) {
	tje.msgCh <- fmt.Sprintf("JawsUpdate(%d)", e.jid)
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
	tje := &testJawsEvent{msgCh: msgCh}

	id := rq.Register(Tag("zomg"), tje, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	rq.inCh <- wsMsg{Data: "text", Jid: id, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-tje.msgCh:
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

	tje.tag = tje
	id2 := rq.Register(tje)
	th.Equal(id2, Jid(2))

	rq.inCh <- wsMsg{Data: "text2", Jid: id2, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-tje.msgCh:
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

	rq.Dirty(tje)
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
	if err := elem.render(&sb, []any{"attr"}); err != nil {
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
