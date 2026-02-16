package core

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
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

func (t *testJawsEvent) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	var tag any
	if tag, err = e.ApplyGetter(t); err == nil {
		w.Write([]byte(fmt.Sprint(params)))
		t.msgCh <- fmt.Sprintf("JawsRender(%d)%#v", e.jid, tag)
	}
	return
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
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}
	zomgItem := &testUi{}
	id := rq.Register(zomgItem, je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	je.clickerr = ErrEventUnhandled
	rq.InCh <- WsMsg{Data: "name", Jid: id, What: what.Click}
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
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}
	zomgItem := &testUi{}
	id := rq.Register(zomgItem, je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	je.clickerr = ErrEventUnhandled
	je.eventerr = ErrEventUnhandled
	rq.InCh <- WsMsg{Data: "name", Jid: id, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != ErrEventUnhandled.Error() {
			t.Error(s)
		}
	}
}

func Test_JawsEvent_NonClickInvokesJawsEventForDualHandler(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)
	je := &testJawsEvent{msgCh: msgCh}
	zomgItem := &testUi{}
	id := rq.Register(zomgItem, je, "attr1", []string{"attr2"}, template.HTMLAttr("attr3"), []template.HTMLAttr{"attr4"})

	rq.InCh <- WsMsg{Data: "typed", Jid: id, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		if s != `JawsEvent: Input "typed"` {
			t.Errorf("unexpected handler call: %q", s)
		}
	}
}

type testJawsEventHandler struct {
	UI
	msgCh    chan string
	eventerr error
}

func (t *testJawsEventHandler) JawsGetHTML(e *Element) template.HTML {
	return "tjEH"
}

func (t *testJawsEventHandler) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if err = t.eventerr; err == nil {
		t.msgCh <- fmt.Sprintf("JawsEvent: %s %q", wht, val)
	} else {
		t.msgCh <- err.Error()
	}
	return
}

type testPanicEventHandler struct {
	panicVal any
}

func (h testPanicEventHandler) JawsEvent(e *Element, wht what.What, val string) error {
	panic(h.panicVal)
}

type testClickCounter struct {
	n         int
	wantName  string
	lastValue string
}

func (c *testClickCounter) JawsClick(_ *Element, name string) error {
	c.lastValue = name
	if name != c.wantName {
		return ErrEventUnhandled
	}
	c.n++
	return nil
}

func Test_CallEventHandlers_PanicError(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	wantErr := fmt.Errorf("boom")
	err := CallEventHandlers(testPanicEventHandler{panicVal: wantErr}, elem, what.Input, "")
	if !errors.Is(err, ErrEventHandlerPanic) {
		t.Errorf("got %v, want ErrEventHandlerPanic", err)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Unwrap: got %v, want %v", errors.Unwrap(err), wantErr)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("Error() = %q, want it to contain %q", err.Error(), "boom")
	}
}

func Test_CallEventHandlers_PanicString(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	err := CallEventHandlers(testPanicEventHandler{panicVal: "oops"}, elem, what.Input, "")
	if !errors.Is(err, ErrEventHandlerPanic) {
		t.Errorf("got %v, want ErrEventHandlerPanic", err)
	}
	if errors.Unwrap(err) != nil {
		t.Errorf("Unwrap: got %v, want nil", errors.Unwrap(err))
	}
	if !strings.Contains(err.Error(), "oops") {
		t.Errorf("Error() = %q, want it to contain %q", err.Error(), "oops")
	}
}

func Test_CallEventHandlers_ClickOnlyHandlerViaApplyGetter(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	clickCounter := &testClickCounter{wantName: "name"}
	if _, err := elem.ApplyGetter(clickCounter); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}

	err := CallEventHandlers(elem.Ui(), elem, what.Click, "name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click handler to be called once, got %d", clickCounter.n)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.Click, "wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click count to stay 1 for wrong name, got %d", clickCounter.n)
	}
}

func Test_CallEventHandlers_ClickOnlyHandlerViaApplyParams(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(testDivWidget{inner: "x"})
	clickCounter := &testClickCounter{wantName: "name"}
	elem.ApplyParams([]any{clickCounter})

	err := CallEventHandlers(elem.Ui(), elem, what.Click, "name")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click handler to be called once, got %d", clickCounter.n)
	}
	err = CallEventHandlers(elem.Ui(), elem, what.Click, "wrong")
	if err != ErrEventUnhandled {
		t.Fatalf("expected ErrEventUnhandled for wrong name, got %v", err)
	}
	if clickCounter.n != 1 {
		t.Fatalf("expected click count to stay 1 for wrong name, got %d", clickCounter.n)
	}
}

func Test_JawsEvent_ExtraHandler(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	msgCh := make(chan string, 1)
	defer close(msgCh)

	je := &testJawsEventHandler{msgCh: msgCh}

	var sb strings.Builder
	elem := rq.NewElement(testDivWidget{inner: "tjEH"})
	th.NoErr(elem.JawsRender(&sb, []any{je}))
	th.Equal(sb.String(), "<div id=\"Jid.1\">tjEH</div>")

	rq.InCh <- WsMsg{Data: "name", Jid: 1, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-msgCh:
		th.Equal(s, "JawsEvent: Click \"name\"")
	}
}
