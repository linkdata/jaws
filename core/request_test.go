package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

const testTimeout = time.Second * 3

func fillWsCh(ch chan WsMsg) {
	for {
		select {
		case ch <- WsMsg{}:
		default:
			return
		}
	}
}

func TestRequest_Registrations(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	x := &testUi{}

	is.Equal(rq.wantMessage(&Message{Dest: x}), false)
	jid := rq.Register(x)
	is.True(jid.IsValid())
	is.Equal(rq.wantMessage(&Message{Dest: x}), true)
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
<script>
document.getElementById("Jid.1")?.setAttribute("hidden","yes");
document.getElementById("Jid.1")?.removeAttribute("hidden");
document.getElementById("Jid.1")?.classList?.add("cls");
document.getElementById("Jid.1")?.classList?.remove("cls");
</script>`, rq.JawsKeyString())
	th.Equal(want, buf.String())

	// verify getTailActions drained the consumed messages from wsQueue
	rq.muQueue.Lock()
	num = len(rq.wsQueue)
	rq.muQueue.Unlock()
	th.Equal(num, 0)
}

func TestRequestWriter_TailHTML_EscapesScriptClose(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)
	item := &testUi{}
	e := rq.NewElement(item)
	e.SetAttr("title", "</script><img onerror=alert(1) src=x>")

	var buf bytes.Buffer
	rq.Writer(&buf).TailHTML()
	s := buf.String()
	if strings.Contains(s, "</script><img") {
		t.Fatalf("getTailActions did not escape </script> in attribute value: %s", s)
	}
	th.True(strings.Contains(s, `\x3c/script>`))
}

func TestRequestWriter_TailHTML_PreservesNonAttrMessages(t *testing.T) {
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

	var buf bytes.Buffer
	rq.Writer(&buf).TailHTML()

	// SAttr and SClass consumed, Value and Inner preserved
	rq.muQueue.Lock()
	th.Equal(len(rq.wsQueue), 2)
	th.Equal(rq.wsQueue[0].What, what.Value)
	th.Equal(rq.wsQueue[1].What, what.Inner)
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
	rq.Jaws.Broadcast(Message{Dest: x, What: what.Inner, Data: "bar"})
	select {
	case <-time.NewTimer(time.Hour).C:
		is.Error("timeout")
	case msg := <-rq.OutCh:
		elem := rq.GetElementByJid(jid)
		is.True(elem != nil)
		if elem != nil {
			is.Equal(msg, WsMsg{Jid: elem.jid, Data: "bar", What: what.Inner})
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
	case rq.InCh <- WsMsg{Jid: jid, What: what.Input, Data: "1"}:
	}
	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- WsMsg{Jid: jid, What: what.Input, Data: "2"}:
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
	rq.Jaws.Broadcast(Message{Dest: x, What: what.Hook, Data: "bar"})

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
	rq.Jaws.Broadcast(Message{Dest: endItem, What: what.Input, Data: ""}) // to know when to stop
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
	rq.Jaws.Broadcast(Message{Dest: fooItem, What: what.Input, Data: "bar"})
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.OutCh:
		th.Fatal(s)
	case <-gotFooCall:
	}

	// fn returning error should send an danger alert message
	rq.Jaws.Broadcast(Message{Dest: errItem, What: what.Input, Data: "omg"})
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.OutCh:
		th.Equal(msg.Format(), (&WsMsg{
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
		rq.Jaws.Broadcast(Message{Dest: sleepItem, What: what.Input, Data: strconv.Itoa(i + 1)})
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
		case rq.InCh <- WsMsg{Jid: jid, What: what.Input}:
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
			rq.Jaws.Broadcast(Message{Dest: spewItem, What: what.Input})
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

	rq.Jaws.Broadcast(Message{Dest: spewItem, What: what.Input})

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
			rq.Jaws.Broadcast(Message{Dest: rq})
		}
		select {
		case rq.InCh <- WsMsg{}:
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
	e11.Tag(Tag("e11"), Tag("foo"))
	e12 := rq1.NewElement(ui1)
	th.Equal(e12.jid, Jid(2))
	e12.Tag(Tag("e12"))
	e13 := rq1.NewElement(ui1)
	th.Equal(e13.jid, Jid(3))
	e13.Tag(Tag("e13"), Tag("bar"))

	rq2 := tj.newRequest(nil)
	ui2 := &testUi{}
	e21 := rq2.NewElement(ui2)
	th.Equal(e21.jid, Jid(4))
	e21.Tag(Tag("e21"), Tag("foo"))
	e22 := rq2.NewElement(ui2)
	th.Equal(e22.jid, Jid(5))
	e22.Tag(Tag("e22"))
	e23 := rq2.NewElement(ui2)
	th.Equal(e23.jid, Jid(6))
	e23.Tag(Tag("e23"))

	tj.Delete([]any{Tag("foo"), Tag("bar"), Tag("nothere"), Tag("e23")})

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

	tj.Broadcast(Message{
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
	case rq.InCh <- WsMsg{What: what.Remove, Jid: 1, Data: "Jid.1"}:
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
	case rq.InCh <- WsMsg{What: what.Click, Data: "name\tJid.1\tJid.2"}:
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
	e.Tag(Tag("zomg"))

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
