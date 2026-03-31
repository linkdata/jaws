package jaws

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/core/assets"
	"github.com/linkdata/jaws/core/tags"
	"github.com/linkdata/jaws/core/wire"
	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

const testTimeout = time.Second * 3

func fillWsCh(ch chan wire.WsMsg) {
	for {
		select {
		case ch <- wire.WsMsg{}:
		default:
			return
		}
	}
}

func TestRequest_MiscBranches(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	reg := testRegisterUI{Updater: &testUi{}}
	elem := rq.NewElement(reg)
	if err := elem.JawsRender(nil, nil); err != nil {
		t.Fatal(err)
	}

	if rq.Request.Initial() == nil {
		t.Fatal("expected initial request")
	}
	if rq.Initial() == nil {
		t.Fatal("expected initial request from writer")
	}

	e2 := rq.NewElement(&testUi{})
	id2 := e2.Jid()
	rq.DeleteElement(e2)
	if rq.GetElementByJid(id2) != nil {
		t.Fatal("expected deleted element")
	}
}

func TestRequest_Registrations(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	x := &testUi{}

	is.Equal(rq.wantMessage(&wire.Message{Dest: x}), false)
	jid := rq.Register(x)
	is.True(jid.IsValid())
	is.Equal(rq.wantMessage(&wire.Message{Dest: x}), true)
}

func TestRequest_HeadHTML(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	var sb strings.Builder
	rq.Writer(&sb).HeadHTML()
	txt := sb.String()
	is.Equal(strings.Contains(txt, rq.JawsKeyString()), true)
	is.Equal(strings.Contains(txt, jw.serveJS.Name), true)
	is.Equal(strings.Count(txt, "<script"), strings.Count(txt, "</script>"))
	is.Equal(strings.Count(txt, "<style>"), strings.Count(txt, "</style>"))
}

func TestRequestWriter_TailHTML(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)
	item := &testUi{}
	e := rq.NewElement(item)
	e.SetAttr("hidden", "yes")
	e.RemoveAttr("hidden")
	e.SetClass("cls")
	e.RemoveClass("cls")

	rq.muQueue.Lock()
	num := len(rq.wsQueue)
	rq.muQueue.Unlock()
	th.Equal(num, 4)

	var buf bytes.Buffer
	rq.Writer(&buf).TailHTML()
	want := fmt.Sprintf(`
<noscript><div class="jaws-alert">This site requires Javascript for full functionality.</div><img src="/jaws/%s/noscript" alt="noscript"></noscript>
<script src="/jaws/.tail/%s"></script>
`, rq.JawsKeyString(), rq.JawsKeyString())
	th.Equal(buf.String(), want)

	// TailHTML should not consume wsQueue messages.
	rq.muQueue.Lock()
	num = len(rq.wsQueue)
	rq.muQueue.Unlock()
	th.Equal(num, 4)
}

func TestRequest_writeTailScript_EscapesScriptClose(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)
	item := &testUi{}
	e := rq.NewElement(item)
	e.SetAttr("title", "</script><img onerror=alert(1) src=x>")

	w := httptest.NewRecorder()
	if err := rq.writeTailScriptResponse(w); err != nil {
		t.Fatal(err)
	}
	s := w.Body.String()
	if strings.Contains(s, "</script><img") {
		t.Fatalf("writeTailScript did not escape </script> in attribute value: %s", s)
	}
	th.True(strings.Contains(s, `\x3c/script>`))
}

func TestRequest_writeTailScript_PreservesNonAttrMessages(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)
	item := &testUi{}
	e := rq.NewElement(item)

	// queue a mix of attribute and non-attribute messages
	e.SetAttr("hidden", "")
	e.SetValue("hello")
	e.SetClass("cls")
	e.SetInner("content")

	rq.muQueue.Lock()
	th.Equal(len(rq.wsQueue), 4)
	rq.muQueue.Unlock()

	w := httptest.NewRecorder()
	if err := rq.writeTailScriptResponse(w); err != nil {
		t.Fatal(err)
	}

	// SAttr and SClass consumed, Value and Inner preserved
	rq.muQueue.Lock()
	th.Equal(len(rq.wsQueue), 2)
	th.Equal(rq.wsQueue[0].What, what.Value)
	th.Equal(rq.wsQueue[1].What, what.Inner)
	rq.muQueue.Unlock()
}

func TestRequest_writeTailScript_RemoveAttrAndClass(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)
	item := &testUi{}
	e := rq.NewElement(item)
	e.RemoveAttr("hidden")
	e.RemoveClass("cls")

	w := httptest.NewRecorder()
	if err := rq.writeTailScriptResponse(w); err != nil {
		t.Fatal(err)
	}
	s := w.Body.String()
	th.True(strings.Contains(s, `removeAttribute("hidden");`))
	th.True(strings.Contains(s, `classList?.remove("cls");`))

	rq.muQueue.Lock()
	th.Equal(len(rq.wsQueue), 0)
	rq.muQueue.Unlock()
}

func TestRequest_SendArrivesOk(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	x := &testUi{}
	jid := rq.Register(x)
	elem := rq.GetElementByJid(jid)
	is.True(elem != nil)
	rq.Jaws.Broadcast(wire.Message{Dest: x, What: what.Inner, Data: "bar"})
	select {
	case <-time.NewTimer(time.Hour).C:
		is.Error("timeout")
	case msg := <-rq.OutCh:
		elem := rq.GetElementByJid(jid)
		is.True(elem != nil)
		if elem != nil {
			is.Equal(msg, wire.WsMsg{Jid: elem.jid, Data: "bar", What: what.Inner})
		}
	}
}

func TestRequest_SetContext(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	type testKey string
	rq.SetContext(func(oldctx context.Context) (newctx context.Context) {
		return context.WithValue(oldctx, testKey("key"), "val")
	})
	if rq.Context().Value(testKey("key")) != "val" {
		t.Fatal("val not set")
	}
}

func TestRequest_SetContext_NilPanics(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	defer func() {
		x := recover()
		if x == nil {
			t.Fatal("expected panic")
		}
		if got := fmt.Sprint(x); got != "context must not be nil" {
			t.Fatalf("unexpected panic %q", got)
		}
	}()

	rq.SetContext(func(context.Context) context.Context { return nil })
}

func TestRequest_SetContextCancellationStopsQueuedEvents(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	block := make(chan struct{})
	started := make(chan struct{}, 1)
	var calls int32

	item := &testUi{}
	rq.Register(item, func(e *Element, evt what.What, val string) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			started <- struct{}{}
			<-block
		}
		return nil
	})
	jid := jidForTag(rq.Request, item)
	if jid == 0 {
		t.Fatal("missing jid")
	}

	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{Jid: jid, What: what.Input, Data: "1"}:
	}
	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{Jid: jid, What: what.Input, Data: "2"}:
	}
	select {
	case <-th.C:
		th.Timeout()
	case <-started:
	}

	rq.SetContext(func(oldctx context.Context) context.Context {
		ctx, cancel := context.WithCancel(oldctx)
		cancel()
		return ctx
	})
	close(block)

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&calls) > 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected queued events to stop after context replacement cancellation, got %d calls", got)
	}
}

func TestRequest_OutboundRespectsContextDone(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	var callCount int32
	x := &testUi{}
	rq.Register(x, func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		rq.cancel(nil)
		return errors.New(val)
	})
	fillWsCh(rq.OutCh)
	rq.Jaws.Broadcast(wire.Message{Dest: x, What: what.Hook, Data: "bar"})

	select {
	case <-th.C:
		th.Equal(int(atomic.LoadInt32(&callCount)), 0)
		th.Timeout()
	case <-rq.Jaws.Done():
		th.Fatal("jaws done too soon")
	case <-rq.ctx.Done():
	}

	th.Equal(int(atomic.LoadInt32(&callCount)), 1)

	select {
	case <-rq.Jaws.Done():
		th.Fatal("jaws done too soon")
	default:
	}
}

func TestRequest_Trigger(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	gotFooCall := make(chan struct{})
	gotEndCall := make(chan struct{})
	fooItem := &testUi{}
	rq.Register(fooItem, func(e *Element, evt what.What, val string) error {
		defer close(gotFooCall)
		return nil
	})
	errItem := &testUi{}
	rq.Register(errItem, func(e *Element, evt what.What, val string) error {
		return errors.New(val)
	})
	endItem := &testUi{}
	rq.Register(endItem, func(e *Element, evt what.What, val string) error {
		defer close(gotEndCall)
		return nil
	})

	// broadcasts from ourselves should not invoke fn
	rq.Jaws.Broadcast(wire.Message{Dest: endItem, What: what.Input, Data: ""}) // to know when to stop
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.OutCh:
		th.Fatal(s)
	case <-gotFooCall:
		th.Fatal("gotFooCall")
	case <-gotEndCall:
	}

	// global broadcast should invoke fn
	rq.Jaws.Broadcast(wire.Message{Dest: fooItem, What: what.Input, Data: "bar"})
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.OutCh:
		th.Fatal(s)
	case <-gotFooCall:
	}

	// fn returning error should send an danger alert message
	rq.Jaws.Broadcast(wire.Message{Dest: errItem, What: what.Input, Data: "omg"})
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.OutCh:
		th.Equal(msg.Format(), (&wire.WsMsg{
			Data: "danger\nomg",
			Jid:  jid.Jid(0),
			What: what.Alert,
		}).Format())
	}
}

func TestRequest_EventFnQueue(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	// calls to slow event functions queue up and are executed in order
	firstDoneCh := make(chan struct{})
	var sleepDone int32
	var callCount int32
	sleepItem := &testUi{}
	rq.Register(sleepItem, func(e *Element, evt what.What, val string) error {
		count := int(atomic.AddInt32(&callCount, 1))
		if val != strconv.Itoa(count) {
			t.Logf("val=%s, count=%d, cap=%d", val, count, cap(rq.OutCh))
			th.Fail()
		}
		if count == 1 {
			close(firstDoneCh)
		}
		for atomic.LoadInt32(&sleepDone) == 0 {
			select {
			case <-t.Context().Done():
				return nil
			default:
				time.Sleep(time.Millisecond)
			}
		}
		return nil
	})

	for i := 0; i < cap(rq.OutCh); i++ {
		rq.Jaws.Broadcast(wire.Message{Dest: sleepItem, What: what.Input, Data: strconv.Itoa(i + 1)})
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-rq.DoneCh:
		th.Fatal("doneCh")
	case <-firstDoneCh:
	}

	th.Equal(atomic.LoadInt32(&callCount), int32(1))
	atomic.StoreInt32(&sleepDone, 1)
	th.Equal(rq.PanicVal, nil)

	for int(atomic.LoadInt32(&callCount)) < cap(rq.OutCh) {
		select {
		case <-th.C:
			t.Logf("callCount=%d, cap=%d", atomic.LoadInt32(&callCount), cap(rq.OutCh))
			th.Equal(rq.PanicVal, nil)
			th.Timeout()
		default:
			time.Sleep(time.Millisecond)
		}
	}
	th.Equal(atomic.LoadInt32(&callCount), int32(cap(rq.OutCh)))
}

func TestRequest_EventFnQueueOverflowPanicsWithNoLogger(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))

	var wait int32

	bombItem := &testUi{}
	rq.Register(bombItem, func(e *Element, evt what.What, val string) error {
		delay := 1 << atomic.AddInt32(&wait, 1)
		select {
		case <-t.Context().Done():
		case <-time.NewTimer(time.Millisecond * time.Duration(min(1000, delay))).C:
		}
		return nil
	})

	rq.ExpectPanic = true
	rq.Jaws.Logger = nil
	jid := jidForTag(rq.Request, bombItem)

	for {
		select {
		case <-rq.DoneCh:
			if t.Context().Err() == nil {
				th.True(rq.Panicked)
				txt := fmt.Sprint(rq.PanicVal)
				if !strings.Contains(txt, "eventCallCh is full sending") {
					t.Log(log.String())
					t.Errorf("unexpected panic value %q", txt)
				}
			} else {
				t.Log(log.String())
				t.Error("test timed out before event channel full")
			}
			return
		case <-th.C:
			th.Timeout()
		case rq.InCh <- wire.WsMsg{Jid: jid, What: what.Input}:
		}
	}
}

func TestRequest_IgnoresIncomingMsgsDuringShutdown(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))

	waitms := 1000
	if deadlock.Debug {
		waitms *= 10
	}

	var spewState int32
	var callCount int32
	spewItem := &testUi{}
	rq.Register(spewItem, func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		if len(rq.OutCh) < cap(rq.OutCh) {
			rq.Jaws.Broadcast(wire.Message{Dest: spewItem, What: what.Input})
			return errors.New("chunks")
		}
		atomic.StoreInt32(&spewState, 1)
		for atomic.LoadInt32(&spewState) == 1 {
			select {
			case <-t.Context().Done():
				atomic.StoreInt32(&spewState, 3)
			default:
				time.Sleep(time.Millisecond)
			}
		}
		atomic.StoreInt32(&spewState, 3)
		return errors.New("chunks")
	})

	fooItem := &testUi{}
	rq.Register(fooItem)

	rq.Jaws.Broadcast(wire.Message{Dest: spewItem, What: what.Input})

	// wait for the event fn to be in hold state
	waited := 0
	for waited < waitms && atomic.LoadInt32(&spewState) == 0 {
		time.Sleep(time.Millisecond)
		waited++
	}
	th.Equal(atomic.LoadInt32(&spewState), int32(1))
	th.Equal(cap(rq.OutCh), len(rq.OutCh))
	th.True(waited < waitms)

	rq.cancel(nil)

	// rq should now be in shutdown phase draining channels
	// while waiting for the event fn to return
	for i := 0; i < cap(rq.OutCh)*2; i++ {
		select {
		case <-rq.DoneCh:
			th.Fatal()
		case <-th.C:
			th.Timeout()
		default:
			rq.Jaws.Broadcast(wire.Message{Dest: rq})
		}
		select {
		case rq.InCh <- wire.WsMsg{}:
		case <-rq.DoneCh:
			th.Fatal()
		case <-th.C:
			th.Timeout()
		}
	}

	// release the event fn
	atomic.StoreInt32(&spewState, 2)

	select {
	case <-rq.DoneCh:
		th.True(atomic.LoadInt32(&spewState) == 3)
		th.True(atomic.LoadInt32(&callCount) > 1)
	case <-th.C:
		t.Logf("timeout callcount %v, spewState %v", atomic.LoadInt32(&callCount), atomic.LoadInt32(&spewState))
		t.Log(log.String())
		th.Timeout()
	}

	// log data should contain message that we were unable to deliver error
	th.True(strings.Contains(log.String(), "outboundMsgCh full sending event"))
}

func TestRequest_Alert(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	rq1.Alert("info", "<html>\nnot\tescaped")
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.OutCh:
		s := msg.Format()
		if s != "Alert\t\t\"info\\n<html>\\nnot\\tescaped\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case s := <-rq2.OutCh:
		t.Errorf("%q", s)
	default:
	}
}

func TestRequest_Redirect(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	rq1.Redirect("some-url")
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.OutCh:
		s := msg.Format()
		if s != "Redirect\t\t\"some-url\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case s := <-rq2.OutCh:
		t.Errorf("%q", s)
	default:
	}
}

func TestRequest_AlertError(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq := tj.newRequest(nil)
	rq.AlertError(errors.New("<html>\nshould-be-escaped"))
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.OutCh:
		s := msg.Format()
		if s != "Alert\t\t\"danger\\n&lt;html&gt;\\nshould-be-escaped\"\n" {
			t.Errorf("%q", s)
		}
	}
}

func TestRequest_DeleteByTag(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	NextJid = 0
	rq1 := tj.newRequest(nil)
	ui1 := &testUi{}
	e11 := rq1.NewElement(ui1)
	th.Equal(e11.jid, Jid(1))
	e11.Tag(tags.Tag("e11"), tags.Tag("foo"))
	e12 := rq1.NewElement(ui1)
	th.Equal(e12.jid, Jid(2))
	e12.Tag(tags.Tag("e12"))
	e13 := rq1.NewElement(ui1)
	th.Equal(e13.jid, Jid(3))
	e13.Tag(tags.Tag("e13"), tags.Tag("bar"))

	rq2 := tj.newRequest(nil)
	ui2 := &testUi{}
	e21 := rq2.NewElement(ui2)
	th.Equal(e21.jid, Jid(4))
	e21.Tag(tags.Tag("e21"), tags.Tag("foo"))
	e22 := rq2.NewElement(ui2)
	th.Equal(e22.jid, Jid(5))
	e22.Tag(tags.Tag("e22"))
	e23 := rq2.NewElement(ui2)
	th.Equal(e23.jid, Jid(6))
	e23.Tag(tags.Tag("e23"))

	tj.Delete([]any{tags.Tag("foo"), tags.Tag("bar"), tags.Tag("nothere"), tags.Tag("e23")})

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.OutCh:
		s := msg.Format()
		if s != "Delete\tJid.1\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.OutCh:
		s := msg.Format()
		if s != "Delete\tJid.3\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.OutCh:
		s := msg.Format()
		if s != "Delete\tJid.4\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.OutCh:
		s := msg.Format()
		if s != "Delete\tJid.6\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}
}

func TestRequest_HTMLIdBroadcast(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	tj.Broadcast(wire.Message{
		Dest: "fooId",
		What: what.Inner,
		Data: "inner",
	})
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.OutCh:
		s := msg.Format()
		if s != "Inner\tfooId\t\"inner\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.OutCh:
		s := msg.Format()
		if s != "Inner\tfooId\t\"inner\"\n" {
			t.Errorf("%q", s)
		}
	}
}

func jidForTag(rq *Request, tag any) jid.Jid {
	if elems := rq.GetElements(tag); len(elems) > 0 {
		return elems[0].jid
	}
	return 0
}

func TestRequest_ConnectFn(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	th.Equal(rq.GetConnectFn(), nil)
	th.NoErr(rq.onConnect())

	wantErr := errors.New("getouttahere")
	fn := func(rq *Request) error {
		return wantErr
	}
	rq.SetConnectFn(fn)
	th.Equal(rq.onConnect(), wantErr)
}

func TestRequest_validateWebSocketOrigin_MatchesInitialRequestOrigin(t *testing.T) {
	tests := []struct {
		name       string
		initialURL string
		origin     string
		wantErr    error
	}{
		{
			name:       "same origin http accepted",
			initialURL: "http://example.test/page",
			origin:     "http://example.test",
			wantErr:    nil,
		},
		{
			name:       "same origin https with non-default port accepted",
			initialURL: "https://example.test:8443/page",
			origin:     "https://example.test:8443",
			wantErr:    nil,
		},
		{
			name:       "same origin https with explicit default port accepted",
			initialURL: "https://example.test/page",
			origin:     "https://example.test:443",
			wantErr:    nil,
		},
		{
			name:       "different host rejected",
			initialURL: "https://example.test/page",
			origin:     "https://evil.test",
			wantErr:    ErrWebsocketOriginWrongHost,
		},
		{
			name:       "different port rejected",
			initialURL: "http://example.test:8080/page",
			origin:     "http://example.test:8081",
			wantErr:    ErrWebsocketOriginWrongHost,
		},
		{
			name:       "different scheme rejected",
			initialURL: "https://example.test/page",
			origin:     "http://example.test",
			wantErr:    ErrWebsocketOriginWrongScheme,
		},
		{
			name:       "missing origin rejected",
			initialURL: "http://example.test/page",
			origin:     "",
			wantErr:    ErrWebsocketOriginMissing,
		},
		{
			name:       "unsupported origin scheme rejected",
			initialURL: "http://example.test/page",
			origin:     "ws://example.test",
			wantErr:    ErrWebsocketOriginWrongScheme,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()

			initialURL, err := url.Parse(tt.initialURL)
			if err != nil {
				t.Fatal(err)
			}
			path := initialURL.Path
			if path == "" {
				path = "/"
			}
			// Server requests don't populate URL.Scheme/URL.Host, only Host.
			initial := httptest.NewRequest(http.MethodGet, path, nil)
			initial.Host = initialURL.Host
			if strings.EqualFold(initialURL.Scheme, "https") {
				initial.TLS = &tls.ConnectionState{}
			}
			rq := jw.NewRequest(initial)
			defer jw.recycle(rq)

			wsReq := httptest.NewRequest(http.MethodGet, "/jaws/"+rq.JawsKeyString(), nil)
			if tt.origin != "" {
				wsReq.Header.Set("Origin", tt.origin)
			}

			err = rq.validateWebSocketOrigin(wsReq)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("validateWebSocketOrigin() error = %v, want nil", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateWebSocketOrigin() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequest_Dirty(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tss1 := &testUi{s: "foo1"}
	tss2 := &testUi{s: "foo2"}
	rq.UI(newTestTextInputWidget(tss1))
	rq.UI(newTestTextInputWidget(tss2))
	th.Equal(tss1.getCalled, int32(1))
	th.Equal(tss2.getCalled, int32(1))
	th.True(strings.Contains(string(rq.BodyString()), "foo1"))
	th.True(strings.Contains(string(rq.BodyString()), "foo2"))

	rq.Dirty(tss1)
	rq.Dirty(tss2)
	for atomic.LoadInt32(&tss1.getCalled) < 2 && atomic.LoadInt32(&tss2.getCalled) < 2 {
		select {
		case <-th.C:
			th.Timeout()
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestRequest_UpdatePanicLogs(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))

	tss := &testUi{
		updateFn: func(e *Element) {
			panic("wildpanic")
		}}
	rq.UI(tss)
	rq.Dirty(tss)
	select {
	case <-th.C:
		th.Timeout()
	case <-rq.DoneCh:
	}
	if s := log.String(); !strings.Contains(s, "wildpanic") {
		t.Error(s)
	}
}

func TestRequest_IncomingRemove(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	tss := newTestSetter("")
	rq.UI(newTestTextInputWidget(tss))

	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{What: what.Remove, Jid: 1, Data: "Jid.1"}:
	}

	elem := rq.GetElementByJid(1)
	for elem != nil {
		select {
		case <-th.C:
			th.Timeout()
		default:
			time.Sleep(time.Millisecond)
			elem = rq.GetElementByJid(1)
		}
	}
}

func TestRequest_IncomingClick(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	tjc1 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}
	tjc1.err = ErrEventUnhandled
	tjc2 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}

	rq.UI(testDivWidget{inner: "1"}, tjc1)
	rq.UI(testDivWidget{inner: "2"}, tjc2)

	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{What: what.Click, Data: "name\tJid.1\tJid.2"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case s := <-tjc2.clickCh:
		if s != "name" {
			t.Error(s)
		}
	}
	select {
	case s := <-tjc1.clickCh:
		t.Errorf("should have been ignored, got %q", s)
	default:
	}
}

func TestRequest_CustomErrors(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	cause := newErrNoWebSocketRequest(rq.Request)
	err := newErrPendingCancelledLocked(rq.Request, cause)
	th.True(errors.Is(err, ErrPendingCancelled))
	th.True(errors.Is(err, ErrNoWebSocketRequest))
	th.Equal(errors.Is(cause, ErrPendingCancelled), false)
	var target1 errNoWebSocketRequest
	th.True(errors.As(err, &target1))
	var target2 errPendingCancelled
	th.Equal(errors.As(cause, &target2), false)
}

func TestRequest_renderDebugLocked(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	tss := &testUi{}
	e := rq.NewElement(tss)
	e.Tag(tags.Tag("zomg"))

	var sb strings.Builder
	e.renderDebug(&sb)

	txt := sb.String()
	is.Equal(strings.Contains(txt, "zomg"), true)
	is.Equal(strings.Contains(txt, "n/a"), false)

	rq.mu.Lock()
	defer rq.mu.Unlock()
	sb.Reset()
	e.renderDebug(&sb)

	txt = sb.String()
	is.Equal(strings.Contains(txt, "zomg"), false)
	is.Equal(strings.Contains(txt, "n/a"), true)
}

func TestCoverage_PendingSubscribeMaintenanceAndParse(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	hr := httptest.NewRequest("GET", "/", nil)
	rq := jw.NewRequest(hr)
	if got := jw.Pending(); got != 1 {
		t.Fatalf("expected one pending request, got %d", got)
	}
	if claimed := jw.UseRequest(rq.JawsKey, hr); claimed != rq {
		t.Fatal("expected request claim")
	}
	if got := jw.Pending(); got != 0 {
		t.Fatalf("expected zero pending requests, got %d", got)
	}

	msgCh := jw.subscribe(rq, 1)
	if msgCh == nil {
		t.Fatal("expected non-nil subscription channel")
	}
	if sub := <-jw.subCh; sub.msgCh != msgCh {
		t.Fatal("unexpected subscription")
	}
	jw.unsubscribe(msgCh)
	if got := <-jw.unsubCh; got != msgCh {
		t.Fatal("unexpected unsubscribe channel")
	}

	// Request timeout path.
	rq.mu.Lock()
	rq.lastWrite = time.Now().Add(-time.Hour)
	rq.mu.Unlock()
	jw.maintenance(time.Second)
	if got := jw.RequestCount(); got != 0 {
		t.Fatalf("expected request recycled, got %d", got)
	}

	// Dead session cleanup path.
	sess := jw.newSession(nil, hr)
	sess.mu.Lock()
	sess.deadline = time.Now().Add(-time.Second)
	sess.mu.Unlock()
	jw.maintenance(time.Second)
	if got := jw.SessionCount(); got != 0 {
		t.Fatalf("expected dead session cleanup, got %d", got)
	}

	// done-channel branch in subscribe and unsubscribe.
	jw.subCh <- subscription{} // fill channel so send case is not selectable
	jw.unsubCh <- make(chan wire.Message)
	jw.Close()
	if ch := jw.subscribe(nil, 1); ch != nil {
		t.Fatalf("expected nil subscription after close, got %v", ch)
	}
	jw.unsubscribe(nil)
}

func TestCoverage_RequestMaintenanceClaimAndErrors(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	hr := httptest.NewRequest("GET", "/", nil)
	rq := jw.NewRequest(hr)
	if err := rq.claim(hr); err != nil {
		t.Fatal(err)
	}
	if err := rq.claim(hr); !errors.Is(err, ErrRequestAlreadyClaimed) {
		t.Fatalf("expected ErrRequestAlreadyClaimed, got %v", err)
	}

	hrA := httptest.NewRequest("GET", "/", nil)
	hrA.RemoteAddr = "1.2.3.4:1234"
	rqA := jw.NewRequest(hrA)
	hrB := httptest.NewRequest("GET", "/", nil)
	hrB.RemoteAddr = "2.2.2.2:4321"
	if err := rqA.claim(hrB); err == nil {
		t.Fatal("expected ip mismatch error")
	}

	now := time.Now()
	rqM := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	rqM.lastWrite = now.Add(-time.Hour)
	if !rqM.maintenance(now, time.Second) {
		t.Fatal("expected maintenance timeout")
	}
	rqR := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	nowR := time.Now()
	rqR.Rendering.Store(true)
	if rqR.maintenance(nowR, time.Hour) {
		t.Fatal("expected maintenance continue")
	}
	rqR.mu.RLock()
	lastWrite := rqR.lastWrite
	rqR.mu.RUnlock()
	if lastWrite != nowR {
		t.Fatalf("expected lastWrite updated to now, got %v want %v", lastWrite, nowR)
	}
	rqC := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	rqC.cancel(errors.New("cancelled"))
	if !rqC.maintenance(time.Now(), time.Hour) {
		t.Fatal("expected maintenance cancellation")
	}
	rqOK := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	rqOK.lastWrite = time.Now()
	if rqOK.maintenance(time.Now(), time.Hour) {
		t.Fatal("expected maintenance keepalive")
	}

	errNoWS := newErrNoWebSocketRequest(rqOK)
	if !errors.Is(errNoWS, ErrNoWebSocketRequest) {
		t.Fatalf("expected no-websocket error type, got %v", errNoWS)
	}
	if got := errNoWS.Error(); !strings.Contains(got, "no WebSocket request received from") {
		t.Fatalf("unexpected error text %q", got)
	}

	maybePanic(nil)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from maybePanic")
		}
	}()
	maybePanic(errors.New("boom"))
}

func TestCoverage_RequestProcessHTTPDoneAndBroadcastDone(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hr := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rq := jw.NewRequest(hr)
	if err := rq.claim(hr); err != nil {
		t.Fatal(err)
	}
	bcastCh := make(chan wire.Message)
	inCh := make(chan wire.WsMsg)
	outCh := make(chan wire.WsMsg, 1)
	done := make(chan struct{})
	go func() {
		rq.process(bcastCh, inCh, outCh)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for process exit on httpDone")
	}

	jw.Close()
	jw.Broadcast(wire.Message{What: what.Update})
}

func TestRequestRecycle_StaleElementIsInert(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	elem := rq.NewElement(testDivWidget{inner: "x"})

	jw.recycle(rq)
	if elem.ui != nil {
		t.Fatal("expected recycled element to have nil ui")
	}
	if got := len(rq.tagMap); got != 0 {
		t.Fatalf("expected no tags in recycled request, got %d", got)
	}
}

func TestRequest_TemplateMissingJid(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug tag not set")
	}
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))
	rq.Jaws.AddTemplateLookuper(template.Must(template.New("badtesttemplate").Parse(`{{with $.Dot}}<div {{$.Attrs}}>{{.}}</div>{{end}}`)))
	if e := rq.Template("badtesttemplate", nil, nil); e != nil {
		t.Error(e)
	}
	if !strings.Contains(log.String(), "WARN") || !strings.Contains(log.String(), "badtesttemplate") {
		t.Error("expected WARN in the log")
		t.Log(log.String())
	}
}

func TestRequest_TemplateJidInsideIf(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug tag not set")
	}
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))
	rq.Jaws.AddTemplateLookuper(template.Must(template.New("iftesttemplate").Parse(`{{with $.Dot}}{{if true}}<div id="{{$.Jid}}" {{$.Attrs}}>{{.}}</div>{{end}}{{end}}`)))
	if e := rq.Template("iftesttemplate", nil, nil); e != nil {
		t.Error(e)
	}
	if strings.Contains(log.String(), "WARN") && strings.Contains(log.String(), "iftesttemplate") {
		t.Error("found WARN in the log")
		t.Log(log.String())
	}
}

func TestRequest_TemplateMissingJidButHasHTMLTag(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug tag not set")
	}
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))
	rq.Jaws.AddTemplateLookuper(template.Must(template.New("badtesttemplate").Parse(`<html>{{with $.Dot}}<div {{$.Attrs}}>{{.}}</div>{{end}}</html>`)))
	if e := rq.Template("badtesttemplate", nil, nil); e != nil {
		t.Error(e)
	}
	if strings.Contains(log.String(), "WARN") {
		t.Error("expected no WARN in the log")
		t.Log(log.String())
	}
}

func TestRequest_Template(t *testing.T) {
	is := newTestHelper(t)

	type intTag int

	type args struct {
		templ  string
		dot    any
		params []any
	}
	tests := []struct {
		name   string
		args   args
		want   template.HTML
		tags   []any
		errtxt string
	}{
		{
			name: "testtemplate",
			args: args{
				"testtemplate",
				intTag(1234),
				[]any{"hidden"},
			},
			want:   `<div id="Jid.1" hidden>1234</div>`,
			tags:   []any{intTag(1234)},
			errtxt: "",
		},
		{
			name: "testtemplate-with-tags",
			args: args{
				"testtemplate",
				tags.Tag("stringtag1"),
				[]any{`style="display: none"`, tags.Tag("stringtag2"), "hidden"},
			},
			want:   `<div id="Jid.1" style="display: none" hidden>stringtag1</div>`,
			tags:   []any{tags.Tag("stringtag1"), tags.Tag("stringtag2")},
			errtxt: "",
		},
	}
	// `{{with $.Dot}}<div id="{{$.Jid}}{{$.Attrs}}">{{.}}</div>{{end}}`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NextJid = 0
			rq := newTestRequest(t)
			defer rq.Close()
			if tt.errtxt != "" {
				defer func() {
					x := recover()
					if e, ok := x.(error); ok {
						if strings.Contains(e.Error(), tt.errtxt) {
							return
						}
					}
					t.Fail()
				}()
			}
			if e := rq.Template(tt.args.templ, tt.args.dot, tt.args.params...); e != nil {
				t.Error(e)
			}
			got := rq.BodyHTML()
			is.Equal(len(rq.elems), 1)
			elem := rq.elems[0]
			if tt.errtxt != "" {
				t.Fail()
			}
			gotTags := elem.Request.TagsOf(elem)
			is.Equal(len(tt.tags), len(gotTags))
			for _, tag := range tt.tags {
				is.True(elem.HasTag(tag))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Request.Template() = %v, want %v", got, tt.want)
			}
		})
	}
}

type templateDot struct {
	clickedCh chan struct{}
	gotName   string
}

func (td *templateDot) JawsClick(e *Element, name string) error {
	defer close(td.clickedCh)
	td.gotName = name
	return nil
}

var _ ClickHandler = &templateDot{}

func TestRequest_Template_Event(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	dot := &templateDot{clickedCh: make(chan struct{})}
	rq.Template("testtemplate", dot)
	rq.Jaws.Broadcast(wire.Message{
		Dest: dot,
		What: what.Update,
	})
	rq.Jaws.Broadcast(wire.Message{
		Dest: dot,
		What: what.Click,
		Data: "foo",
	})
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-dot.clickedCh:
	}
	is.Equal(dot.gotName, "foo")
}

func nextOutboundMsg(t *testing.T, rq *testRequest) wire.WsMsg {
	t.Helper()
	select {
	case msg := <-rq.OutCh:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for outbound ws message")
		return wire.WsMsg{}
	}
}

func TestRequest_IncomingRemoveDoesNotDeleteMessageJid(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})

	select {
	case rq.InCh <- wire.WsMsg{What: what.Remove, Jid: elem.Jid(), Data: ""}:
	case <-time.After(time.Second):
		t.Fatal("timeout sending incoming Remove message")
	}

	select {
	case <-time.After(20 * time.Millisecond):
	case <-rq.DoneCh:
		t.Fatal("request shut down unexpectedly")
	}
	if got := rq.GetElementByJid(elem.Jid()); got == nil {
		t.Fatalf("element %s should still exist after Remove with empty data", elem.Jid())
	}
}

func TestRequest_ReplaceMessageTargetsElementHTML(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tag := &testUi{}
	jid := rq.Register(tag)
	html := `<div id="` + jid.String() + `">replaced</div>`

	rq.Jaws.Replace(tag, html)
	msg := nextOutboundMsg(t, rq)

	if msg.What != what.Replace {
		t.Fatalf("unexpected message type %v", msg.What)
	}
	if msg.Data != html {
		t.Fatalf("replace payload mismatch: got %q want %q", msg.Data, html)
	}
}

func TestRequest_JsCallProducesJawsJSFrameSafeWireData(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tag := &testUi{}
	rq.Register(tag)

	tests := []struct {
		name    string
		jsonstr string
	}{
		{
			name:    "pretty json with newline",
			jsonstr: "{\n\"a\":1}",
		},
		{
			name:    "pretty json with tab",
			jsonstr: "{\t\"a\":1}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq.Jaws.JsCall(tag, "fn", tt.jsonstr)
			msg := nextOutboundMsg(t, rq)
			wire := msg.Format()

			if got := strings.Count(wire, "\n"); got != 1 {
				t.Fatalf("wire message contains embedded newlines (%d): %q", got, wire)
			}
			if got := strings.Count(wire, "\t"); got != 2 {
				t.Fatalf("wire message contains embedded tab separators (%d): %q", got, wire)
			}
		})
	}
}

func TestRequest_JsCallFunctionPathDoesNotBreakWireFraming(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tag := &testUi{}
	rq.Register(tag)

	tests := []struct {
		name   string
		jsfunc string
	}{
		{
			name:   "tab in function path",
			jsfunc: "fn\tpart",
		},
		{
			name:   "newline in function path",
			jsfunc: "fn\npart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq.Jaws.JsCall(tag, tt.jsfunc, `{"a":1}`)
			msg := nextOutboundMsg(t, rq)
			wire := msg.Format()

			if got := strings.Count(wire, "\n"); got != 1 {
				t.Fatalf("wire message contains embedded newlines (%d): %q", got, wire)
			}
			if got := strings.Count(wire, "\t"); got != 2 {
				t.Fatalf("wire message contains embedded tab separators (%d): %q", got, wire)
			}
		})
	}
}

func TestRequest_IncomingRemoveWithZeroContainerJidIsIgnored(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})

	select {
	case rq.InCh <- wire.WsMsg{What: what.Remove, Jid: 0, Data: elem.Jid().String()}:
	case <-time.After(time.Second):
		t.Fatal("timeout sending incoming Remove message")
	}

	select {
	case <-time.After(20 * time.Millisecond):
	case <-rq.DoneCh:
		t.Fatal("request shut down unexpectedly")
	}
	if got := rq.GetElementByJid(elem.Jid()); got == nil {
		t.Fatalf("element %s should not be deletable through zero-container Remove", elem.Jid())
	}
}

type testServer struct {
	jw          *Jaws
	ctx         context.Context
	cancel      context.CancelFunc
	hr          *http.Request
	rr          *httptest.ResponseRecorder
	rq          *Request
	sess        *Session
	srv         *httptest.Server
	connectedCh chan struct{}
}

func newTestServer() (ts *testServer) {
	jw, _ := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	sess := jw.NewSession(rr, hr)
	rq := jw.NewRequest(hr)
	if rq != jw.UseRequest(rq.JawsKey, hr) {
		panic("UseRequest failed")
	}
	ts = &testServer{
		jw:          jw,
		ctx:         ctx,
		cancel:      cancel,
		hr:          hr,
		rr:          rr,
		rq:          rq,
		sess:        sess,
		connectedCh: make(chan struct{}),
	}
	rq.SetConnectFn(ts.connected)
	ts.srv = httptest.NewServer(ts)
	ts.setInitialRequestOrigin()
	return
}

func (ts *testServer) connected(rq *Request) error {
	if rq == ts.rq {
		close(ts.connectedCh)
	}
	return nil
}

func (ts *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/jaws/") {
		jawsKey := assets.JawsKeyValue(strings.TrimPrefix(r.URL.Path, "/jaws/"))
		if rq := ts.jw.UseRequest(jawsKey, r); rq != nil {
			rq.ServeHTTP(w, r)
			return
		}
	}
	ts.rq.ServeHTTP(w, r)
}

func (ts *testServer) Path() string {
	return "/jaws/" + ts.rq.JawsKeyString()
}

func (ts *testServer) Url() string {
	return ts.srv.URL + ts.Path()
}

func (ts *testServer) setInitialRequestOrigin() {
	if ts.hr == nil {
		return
	}
	u, err := url.Parse(ts.srv.URL)
	if err != nil {
		return
	}
	ts.hr.Host = u.Host
	if ts.hr.URL != nil {
		ts.hr.URL.Host = u.Host
		ts.hr.URL.Scheme = u.Scheme
	}
}

func (ts *testServer) origin() string {
	scheme := "http"
	if ts.hr != nil && ts.hr.URL != nil && ts.hr.URL.Scheme != "" {
		scheme = ts.hr.URL.Scheme
	}
	host := ""
	if ts.hr != nil {
		host = ts.hr.Host
	}
	if host == "" {
		if u, err := url.Parse(ts.srv.URL); err == nil {
			host = u.Host
			if scheme == "" {
				scheme = u.Scheme
			}
		}
	}
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + host
}

func (ts *testServer) Dial() (*websocket.Conn, *http.Response, error) {
	hdr := http.Header{}
	hdr.Set("Origin", ts.origin())
	opts := &websocket.DialOptions{HTTPHeader: hdr}
	return websocket.Dial(ts.ctx, ts.Url(), opts)
}

func (ts *testServer) Close() {
	ts.cancel()
	ts.srv.Close()
	ts.jw.Close()
}

func TestWS_UpgradeRequired(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	w := httptest.NewRecorder()
	hr := httptest.NewRequest("", "/", nil)
	rq := jw.NewRequest(hr)
	jw.UseRequest(rq.JawsKey, hr)
	req := httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	rq.ServeHTTP(w, req)
	if w.Code != http.StatusUpgradeRequired {
		t.Error(w.Code)
	}
}

func TestWS_RejectsMissingOrigin(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("expected handshake to be rejected")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestWS_RejectsCrossOrigin(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	hdr := http.Header{}
	hdr.Set("Origin", "https://evil.invalid")
	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), &websocket.DialOptions{HTTPHeader: hdr})
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("expected handshake to be rejected")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestWS_ConnectFnFails(t *testing.T) {
	const nope = "nope"
	ts := newTestServer()
	defer ts.Close()
	ts.rq.SetConnectFn(func(_ *Request) error { return errors.New(nope) })

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if conn != nil {
		defer conn.Close(websocket.StatusNormalClosure, "")
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}
	mt, b, err := conn.Read(ts.ctx)
	if err != nil {
		t.Error(err)
	}
	if mt != websocket.MessageText {
		t.Error(mt)
	}
	if !strings.Contains(string(b), nope) {
		t.Error(string(b))
	}
}

func TestWS_NormalExchange(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	fooError := errors.New("this foo failed")

	gotCallCh := make(chan struct{})
	fooItem := &testUi{}
	testRequestWriter{rq: ts.rq, Writer: httptest.NewRecorder()}.Register(fooItem, func(e *Element, evt what.What, val string) error {
		close(gotCallCh)
		return fooError
	})

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	msg := wire.WsMsg{Jid: jidForTag(ts.rq, fooItem), What: what.Input}
	ctx, cancel := context.WithTimeout(ts.ctx, testTimeout)
	defer cancel()

	err = conn.Write(ctx, websocket.MessageText, msg.Append(nil))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-th.C:
		th.Timeout()
	case <-gotCallCh:
	}

	mt, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mt != websocket.MessageText {
		t.Error(mt)
	}
	var m2 wire.WsMsg
	m2.FillAlert(fooError)
	if !bytes.Equal(b, m2.Append(nil)) {
		t.Error(b)
	}
}
