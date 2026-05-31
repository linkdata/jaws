package jaws

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
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
	"testing/synctest"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
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
	is.NoErr(rq.Writer(&sb).HeadHTML())
	txt := sb.String()
	is.Equal(strings.Contains(txt, rq.JawsKeyString()), true)
	is.Equal(strings.Contains(txt, jw.serveJS.Name), true)
	is.Equal(strings.Contains(txt, `meta name="jawsDebug"`), false)
	is.Equal(strings.Count(txt, "<script"), strings.Count(txt, "</script>"))
	is.Equal(strings.Count(txt, "<style>"), strings.Count(txt, "</style>"))
}

func TestRequest_HeadHTML_DebugMeta(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.Debug = true
	if err = jw.GenerateHeadHTML(); err != nil {
		t.Fatal(err)
	}
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	var sb strings.Builder
	if err = rq.Writer(&sb).HeadHTML(); err != nil {
		t.Fatal(err)
	}
	txt := sb.String()
	if !strings.Contains(txt, `meta name="jawsDebug"`) {
		t.Fatalf("expected debug meta in head html, got %q", txt)
	}
}

func TestRequestWriter_TailHTML(t *testing.T) {
	th := newTestHelper(t)
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
	th.NoErr(rq.Writer(&buf).TailHTML())
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
	rq.SetContext(func(oldCtx context.Context) (newCtx context.Context) {
		return context.WithValue(oldCtx, testKey("key"), "val")
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

	// No Logger is configured, so reportMisuse panics in both debug and production.
	defer func() {
		x := recover()
		if x == nil {
			t.Fatal("expected panic")
		}
		if got := fmt.Sprint(x); !strings.Contains(got, "SetContext function returned a nil context") {
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
	rq.Register(item, func(elem *Element, value string) error {
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

	rq.SetContext(func(oldCtx context.Context) context.Context {
		ctx, cancel := context.WithCancel(oldCtx)
		cancel()
		return ctx
	})
	close(block)

	// Negative assertion: confirm the queued second event never fires after the
	// context is replaced and cancelled. This proves absence over elapsed time, so
	// it intentionally waits on the real clock rather than running in a synctest
	// bubble.
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
	rq.Register(x, func(elem *Element, value string) error {
		atomic.AddInt32(&callCount, 1)
		rq.cancel(nil)
		return errors.New(value)
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
	rq.Register(fooItem, func(elem *Element, value string) error {
		defer close(gotFooCall)
		return nil
	})
	errItem := &testUi{}
	rq.Register(errItem, func(elem *Element, value string) error {
		return errors.New(value)
	})
	endItem := &testUi{}
	rq.Register(endItem, func(elem *Element, value string) error {
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

// TestRequest_EventFnQueue uses an event handler that deliberately blocks
// (busy-waiting on sleepDone) to fill the event queue. That busy-wait is the
// device under test and keeps re-arming the clock, so this test is not suited to
// a synctest bubble; it stays on the real clock with deadline-bounded waits.
func TestRequest_EventFnQueue(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	// calls to slow event functions queue up and are executed in order
	firstDoneCh := make(chan struct{})
	var sleepDone int32
	var callCount int32
	sleepItem := &testUi{}
	rq.Register(sleepItem, func(elem *Element, value string) error {
		count := int(atomic.AddInt32(&callCount, 1))
		if value != strconv.Itoa(count) {
			t.Logf("val=%s, count=%d, cap=%d", value, count, cap(rq.OutCh))
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

// TestRequest_EventFnQueueOverflowCancelsRequest verifies that when a client floods
// events faster than a slow handler can drain them, the request is cancelled rather
// than silently dropping events and limping on with inconsistent state, and that it
// does not panic even with no Logger configured (the default).
func TestRequest_EventFnQueueOverflowCancelsRequest(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	var wait int32

	bombItem := &testUi{}
	rq.Register(bombItem, func(elem *Element, value string) error {
		delay := 1 << atomic.AddInt32(&wait, 1)
		select {
		case <-t.Context().Done():
		case <-time.NewTimer(time.Millisecond * time.Duration(min(1000, delay))).C:
		}
		return nil
	})

	// No Logger configured (the default): the overflow back-pressure must cancel
	// the request without panicking. ExpectPanic stays false, so any panic that
	// does escape is re-raised by the harness and fails the test.
	rq.Jaws.Logger = nil
	jid := jidForTag(rq.Request, bombItem)

	for {
		select {
		case <-rq.DoneCh:
			if t.Context().Err() != nil {
				t.Error("test timed out before event channel full")
			}
			th.True(!rq.Panicked)
			return
		case <-th.C:
			th.Timeout()
		case rq.InCh <- wire.WsMsg{Jid: jid, What: what.Input}:
		}
	}
}

// TestRequest_ClaimRefreshesLastWriteAndStartServeGuards verifies that claim()
// refreshes lastWrite so a request claimed long after its initial render is not
// treated as idle and recycled before ServeHTTP sets running, and that startServe()
// refuses a request that was recycled (clearLocked resets claimed) rather than
// driving a dead, pooled *Request.
func TestRequest_ClaimRefreshesLastWriteAndStartServeGuards(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)

	// Simulate a page that rendered long ago (idle) before its WebSocket connects.
	rq.mu.Lock()
	rq.lastWrite = time.Now().Add(-time.Hour)
	rq.mu.Unlock()

	if jw.UseRequest(rq.JawsKey, hr) != rq {
		t.Fatal("expected claim to succeed")
	}
	// claim refreshed lastWrite, so the just-claimed request is not idle-eligible.
	if rq.maintenance(time.Now(), time.Second) {
		t.Fatal("a freshly claimed request must not be treated as idle by maintenance")
	}

	// If a request is recycled anyway, startServe must refuse to drive it.
	jw.recycle(rq)
	if rq.startServe() {
		t.Fatal("startServe must refuse a recycled request")
	}
}

// TestRequest_IgnoresIncomingMsgsDuringShutdown uses an event handler that
// deliberately blocks (busy-waiting on spewState) while the request shuts down.
// That busy-wait is the device under test and keeps re-arming the clock, so this
// test is not suited to a synctest bubble; it stays on the real clock with
// deadline-bounded waits.
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
	rq.Register(spewItem, func(elem *Element, value string) error {
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

	rq1.Alert("info", "<html>\nis\tescaped")
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.OutCh:
		s := msg.Format()
		if s != "Alert\t\t\"info\\n&lt;html&gt;\\nis\\tescaped\"\n" {
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

func TestRequest_Cancel(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rq.Cancel(errors.New("abort"))
	if rq.Context().Err() == nil {
		t.Error("expected context to be cancelled after Cancel")
	}
}

func TestDefaultAuth_IsAdminWarnsOnceWithLogger(t *testing.T) {
	var buf bytes.Buffer
	da := &DefaultAuth{Logger: slog.New(slog.NewTextHandler(&buf, nil))}
	if !da.IsAdmin() {
		t.Error("DefaultAuth.IsAdmin must return true")
	}
	if !strings.Contains(buf.String(), "DefaultAuth.IsAdmin returns true") {
		t.Errorf("expected the fail-open warning, got %q", buf.String())
	}
	// The warning fires at most once (sync.Once).
	buf.Reset()
	if !da.IsAdmin() {
		t.Error("DefaultAuth.IsAdmin must still return true")
	}
	if buf.Len() != 0 {
		t.Errorf("warning should fire only once, got %q", buf.String())
	}
}

func Test_isSafeRedirect(t *testing.T) {
	for _, s := range []string{"", "/", "/next", "some-url", "http://example.test/x", "https://example.test/x", "HTTPS://EX/x"} {
		if _, ok := isSafeRedirect(s); !ok {
			t.Errorf("expected safe redirect: %q", s)
		}
	}
	// Protocol-relative URLs reach an arbitrary external origin and must be unsafe,
	// alongside script-bearing schemes. Browsers strip leading/trailing whitespace
	// and treat '\' as '/', so whitespace- and backslash-prefixed protocol-relative
	// URLs are bypasses that must also be rejected.
	for _, s := range []string{
		"//host/path", "//evil.com", "javascript:alert(1)", "JavaScript:alert(1)",
		"data:text/html,<script>x</script>", "vbscript:msgbox(1)",
		" //evil.com", "  \t //evil.com  ", "///evil.com",
		"/\\evil.com", "\\/\\/evil.com", "/\\/evil.com",
	} {
		if _, ok := isSafeRedirect(s); ok {
			t.Errorf("expected unsafe redirect: %q", s)
		}
	}
	// Leading/trailing whitespace is trimmed from the value that is sent.
	if safe, ok := isSafeRedirect("  /trim/me  "); !ok || safe != "/trim/me" {
		t.Errorf("expected trimmed safe redirect %q, got %q ok=%v", "/trim/me", safe, ok)
	}
}

func TestRequest_Redirect_unsafeRefused(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rq.Redirect("javascript:alert(1)")
	if logger.err == nil || !strings.Contains(logger.err.Error(), "refusing unsafe redirect") {
		t.Fatalf("expected unsafe redirect to be logged and skipped, got %v", logger.err)
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
	rq1 := tj.newRequest(nil)
	ui1 := &testUi{}
	e11 := rq1.NewElement(ui1)
	th.Equal(e11.jid, Jid(1))
	e11.Tag(tag.Tag("e11"), tag.Tag("foo"))
	e12 := rq1.NewElement(ui1)
	th.Equal(e12.jid, Jid(2))
	e12.Tag(tag.Tag("e12"))
	e13 := rq1.NewElement(ui1)
	th.Equal(e13.jid, Jid(3))
	e13.Tag(tag.Tag("e13"), tag.Tag("bar"))

	rq2 := tj.newRequest(nil)
	ui2 := &testUi{}
	e21 := rq2.NewElement(ui2)
	th.Equal(e21.jid, Jid(1))
	e21.Tag(tag.Tag("e21"), tag.Tag("foo"))
	e22 := rq2.NewElement(ui2)
	th.Equal(e22.jid, Jid(2))
	e22.Tag(tag.Tag("e22"))
	e23 := rq2.NewElement(ui2)
	th.Equal(e23.jid, Jid(3))
	e23.Tag(tag.Tag("e23"))

	tj.Delete([]any{tag.Tag("foo"), tag.Tag("bar"), tag.Tag("nothere"), tag.Tag("e23")})

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
		if s != "Delete\tJid.1\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.OutCh:
		s := msg.Format()
		if s != "Delete\tJid.3\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}
}

func TestRequest_JidsAreRequestScoped(t *testing.T) {
	tj := newTestJaws()
	defer tj.Close()

	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	e11 := rq1.NewElement(&testUi{})
	e12 := rq1.NewElement(&testUi{})
	e21 := rq2.NewElement(&testUi{})
	e22 := rq2.NewElement(&testUi{})

	if e11.Jid() != 1 || e12.Jid() != 2 {
		t.Fatalf("request 1 Jids = %v, %v; want 1, 2", e11.Jid(), e12.Jid())
	}
	if e21.Jid() != 1 || e22.Jid() != 2 {
		t.Fatalf("request 2 Jids = %v, %v; want 1, 2", e21.Jid(), e22.Jid())
	}
}

func TestRequest_JidsAreNotReusedAfterDelete(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e1 := rq.NewElement(&testUi{})
	rq.DeleteElement(e1)
	e2 := rq.NewElement(&testUi{})

	if e1.Jid() != 1 || e2.Jid() != 2 {
		t.Fatalf("Jids = %v, %v; want deleted id 1 and new id 2", e1.Jid(), e2.Jid())
	}
}

func TestRequest_RequestScopedEventIsolation(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()

	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	tjc1 := &testJawsClick{
		clickCh:    make(chan string, 1),
		testSetter: newTestSetter(""),
	}
	tjc2 := &testJawsClick{
		clickCh:    make(chan string, 1),
		testSetter: newTestSetter(""),
	}

	rq1.NewElement(testDivWidget{inner: "one"}).AddHandlers(tjc1)
	rq2.NewElement(testDivWidget{inner: "two"}).AddHandlers(tjc2)

	select {
	case <-th.C:
		th.Timeout()
	case rq2.InCh <- wire.WsMsg{What: what.Click, Jid: 1, Data: "1 2 0 scoped"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case name := <-tjc2.clickCh:
		if name != "scoped" {
			t.Fatalf("unexpected request 2 click name %q", name)
		}
	}
	select {
	case name := <-tjc1.clickCh:
		t.Fatalf("request 1 received request 2 event %q", name)
	default:
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

func jidForTag(rq *Request, tagValue any) jid.Jid {
	if elems := rq.GetElements(tagValue); len(elems) > 0 {
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

func TestRequest_validateWebSocketOrigin_NoInitialFailsClosed(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	// A Request can be constructed without an initial HTTP request (NewRequest(nil)).
	// Origin validation must then fail closed rather than accepting any Origin.
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	wsReq := httptest.NewRequest(http.MethodGet, "/jaws/"+rq.JawsKeyString(), nil)
	wsReq.Header.Set("Origin", "https://example.test")
	if err := rq.validateWebSocketOrigin(wsReq); !errors.Is(err, ErrWebsocketOriginNoInitial) {
		t.Fatalf("validateWebSocketOrigin() with nil initial = %v, want %v", err, ErrWebsocketOriginNoInitial)
	}
}

func TestRequest_Log(t *testing.T) {
	wantErr := errors.New("request log test")

	if got := (*Request)(nil).Log(wantErr); !errors.Is(got, wantErr) {
		t.Fatalf("(*Request)(nil).Log() = %v, want %v", got, wantErr)
	}

	var log bytes.Buffer
	rq := &Request{
		Jaws: &Jaws{
			Logger: slog.New(slog.NewTextHandler(&log, nil)),
		},
	}
	if got := rq.Log(wantErr); !errors.Is(got, wantErr) {
		t.Fatalf("Request.Log() = %v, want %v", got, wantErr)
	}
	if s := log.String(); !strings.Contains(s, wantErr.Error()) {
		t.Fatalf("Request.Log() did not write error to logger: %q", s)
	}
}

func TestRequest_Dirty(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		th := newTestHelper(t)
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		tss1 := &testUi{s: "foo1"}
		tss2 := &testUi{s: "foo2"}
		th.NoErr(rq.UI(newTestTextInputWidget(tss1)))
		th.NoErr(rq.UI(newTestTextInputWidget(tss2)))
		th.Equal(tss1.getCalled, int32(1))
		th.Equal(tss2.getCalled, int32(1))
		th.True(strings.Contains(string(rq.BodyString()), "foo1"))
		th.True(strings.Contains(string(rq.BodyString()), "foo2"))

		rq.Dirty(tss1)
		rq.Dirty(tss2)
		// Dirtying marks the elements; the Serve loop broadcasts what.Update only
		// when its updateTicker fires (1ms in tests). Advance the fake clock past
		// it, then let the process loop re-render both elements (JawsGet again).
		time.Sleep(2 * time.Millisecond)
		synctest.Wait()
		th.True(atomic.LoadInt32(&tss1.getCalled) >= 2)
		th.True(atomic.LoadInt32(&tss2.getCalled) >= 2)
	})
}

func TestRequest_UpdatePanicLogs(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	var log bytes.Buffer
	rq.Jaws.Logger = slog.New(slog.NewTextHandler(&log, nil))

	tss := &testUi{
		updateFn: func(elem *Element) {
			panic("wildpanic")
		}}
	th.NoErr(rq.UI(tss))
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
	synctest.Test(t, func(t *testing.T) {
		th := newTestHelper(t)
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		tss := newTestSetter("")
		th.NoErr(rq.UI(newTestTextInputWidget(tss)))

		// Send the incoming Remove and let the process loop handle it; the
		// element is gone once every bubbled goroutine is durably blocked again.
		rq.InCh <- wire.WsMsg{What: what.Remove, Jid: 1, Data: "Jid.1"}
		synctest.Wait()
		th.Equal(rq.GetElementByJid(1), (*Element)(nil))
	})
}

func TestRequest_IncomingClick(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tjc1 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}
	tjc1.SetErr(ErrEventUnhandled)
	tjc2 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}

	th.NoErr(rq.UI(testDivWidget{inner: "1"}, tjc1))
	th.NoErr(rq.UI(testDivWidget{inner: "2"}, tjc2))

	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{What: what.Click, Data: "1 2 0 name\tJid.1\tJid.2"}:
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

func TestRequest_IncomingClick_WrappedUnhandled(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tjc1 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}
	tjc1.SetErr(fmt.Errorf("wrapped: %w", ErrEventUnhandled))
	tjc2 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}

	th.NoErr(rq.UI(testDivWidget{inner: "1"}, tjc1))
	th.NoErr(rq.UI(testDivWidget{inner: "2"}, tjc2))

	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{What: what.Click, Data: "1 2 0 name\tJid.1\tJid.2"}:
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

func TestRequest_IncomingContextMenu(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tjc1 := &testJawsContextMenu{
		clickCh:    make(chan Click, 2),
		testSetter: newTestSetter(Click{}),
	}
	tjc1.SetErr(ErrEventUnhandled)
	tjc2 := &testJawsContextMenu{
		clickCh:    make(chan Click, 2),
		testSetter: newTestSetter(Click{}),
	}

	th.NoErr(rq.UI(testDivWidget{inner: "1"}, tjc1))
	th.NoErr(rq.UI(testDivWidget{inner: "2"}, tjc2))

	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{What: what.ContextMenu, Data: "10 20 5 name\tJid.1\tJid.2"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case got := <-tjc2.clickCh:
		if got != (Click{Name: "name", X: 10, Y: 20, Shift: true, Alt: true}) {
			t.Error(got)
		}
	}
	select {
	case got := <-tjc1.clickCh:
		t.Errorf("should have been ignored, got %#v", got)
	default:
	}
}

func TestRequest_IncomingContextMenu_WrappedUnhandled(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tjc1 := &testJawsContextMenu{
		clickCh:    make(chan Click, 2),
		testSetter: newTestSetter(Click{}),
	}
	tjc1.SetErr(fmt.Errorf("wrapped: %w", ErrEventUnhandled))
	tjc2 := &testJawsContextMenu{
		clickCh:    make(chan Click, 2),
		testSetter: newTestSetter(Click{}),
	}

	th.NoErr(rq.UI(testDivWidget{inner: "1"}, tjc1))
	th.NoErr(rq.UI(testDivWidget{inner: "2"}, tjc2))

	select {
	case <-th.C:
		th.Timeout()
	case rq.InCh <- wire.WsMsg{What: what.ContextMenu, Data: "10 20 5 name\tJid.1\tJid.2"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case got := <-tjc2.clickCh:
		if got != (Click{Name: "name", X: 10, Y: 20, Shift: true, Alt: true}) {
			t.Error(got)
		}
	}
	select {
	case got := <-tjc1.clickCh:
		t.Errorf("should have been ignored, got %#v", got)
	default:
	}
}

func TestRequest_CustomErrors(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	cause := newErrNoWebSocketRequest(rq.Request)
	err := newErrRequestCancelledLocked(rq.Request, cause)
	th.True(errors.Is(err, ErrRequestCancelled))
	th.True(errors.Is(err, ErrNoWebSocketRequest))
	th.Equal(errors.Is(cause, ErrRequestCancelled), false)
	if errtxt := err.Error(); !strings.Contains(errtxt, cause.Error()) {
		t.Error(errtxt)
	}
	var target1 errNoWebSocketRequest
	th.True(errors.As(err, &target1))
	var target2 errRequestCancelled
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
	e.Tag(tag.Tag("zomg"))

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
	total, active := jw.RequestCounts()
	if total != 1 || active != 0 {
		t.Fatalf("RequestCounts() = %d, %d, want 1, 0", total, active)
	}
	if got := jw.RequestCount(); got != total {
		t.Fatalf("RequestCount() = %d, want %d", got, total)
	}
	if got := jw.Pending(); got != 1 {
		t.Fatalf("expected one pending request, got %d", got)
	}
	if claimed := jw.UseRequest(rq.JawsKey, hr); claimed != rq {
		t.Fatal("expected request claim")
	}
	total, active = jw.RequestCounts()
	if total != 1 || active != 0 {
		t.Fatalf("RequestCounts() = %d, %d, want 1, 0", total, active)
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

func TestRequest_Template(t *testing.T) {
	is := newTestHelper(t)

	type intTag int

	type args struct {
		outer  string
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
				"div",
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
				"div",
				"testtemplate",
				tag.Tag("stringtag1"),
				[]any{`style="display: none"`, tag.Tag("stringtag2"), "hidden"},
			},
			want:   `<div id="Jid.1" style="display: none" hidden>stringtag1</div>`,
			tags:   []any{tag.Tag("stringtag1"), tag.Tag("stringtag2")},
			errtxt: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			if e := rq.Template(tt.args.outer, tt.args.templ, tt.args.dot, tt.args.params...); e != nil {
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
			for _, tagValue := range tt.tags {
				is.True(elem.HasTag(tagValue))
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

func (td *templateDot) JawsClick(elem *Element, click Click) error {
	defer close(td.clickedCh)
	td.gotName = click.Name
	return nil
}

var _ ClickHandler = &templateDot{}

func TestRequest_Template_Event(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	dot := &templateDot{clickedCh: make(chan struct{})}
	is.NoErr(rq.Template("div", "testtemplate", dot))
	rq.Jaws.Broadcast(wire.Message{
		Dest: dot,
		What: what.Update,
	})
	rq.Jaws.Broadcast(wire.Message{
		Dest: dot,
		What: what.Click,
		Data: "1 2 0 foo",
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

func TestRequest_NewElement_DebugComparableCheck(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug checks not enabled")
	}

	rq := newTestRequest(t)
	defer rq.Close()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for non-comparable UI when comparable check is enabled")
		}
	}()
	rq.NewElement(testUnhashableUI{m: map[string]int{"x": 1}})
}

func TestRequest_IncomingRemoveDoesNotDeleteMessageJid(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		elem := rq.NewElement(&testUi{})

		// A Remove whose WebSocket Jid is the message's own element (with empty
		// data, i.e. no child IDs) must not delete that element. synctest.Wait
		// blocks until the process loop has actually handled the message, so the
		// survival assertion is not vacuous.
		rq.InCh <- wire.WsMsg{What: what.Remove, Jid: elem.Jid(), Data: ""}
		synctest.Wait()
		if got := rq.GetElementByJid(elem.Jid()); got == nil {
			t.Fatalf("element %s should still exist after Remove with empty data", elem.Jid())
		}
	})
}

func TestRequest_ReplaceMessageTargetsElementHTML(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tagValue := &testUi{}
	jid := rq.Register(tagValue)
	html := template.HTML(`<div id="` + jid.String() + `">replaced</div>`)

	rq.Jaws.Replace(tagValue, html)
	msg := nextOutboundMsg(t, rq)

	if msg.What != what.Replace {
		t.Fatalf("unexpected message type %v", msg.What)
	}
	if msg.Data != string(html) {
		t.Fatalf("replace payload mismatch: got %q want %q", msg.Data, html)
	}
}

func TestRequest_StringIdBroadcastDataIsJSONParseable(t *testing.T) {
	// A broadcast to a plain HTML-id string must be quoted JSON-safely. strconv.Quote
	// emits \xNN / \UXXXXXXXX escapes (for control bytes, DEL and invalid UTF-8) that
	// the browser's JSON.parse rejects, which would drop every coalesced update in the
	// same WebSocket frame.
	tests := []struct {
		name string
		data string
		want string // expected decoded value; "" means only require that it parses
	}{
		{"control byte", "before\x01after", "before\x01after"},
		{"DEL byte", "before\x7fafter", "before\x7fafter"},
		{"angle brackets", "<script>x</script>", "<script>x</script>"},
		{"invalid utf8", "before\xff\xfeafter", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq := newTestRequest(t)
			defer rq.Close()

			rq.Jaws.SetInner("some-id", template.HTML(tt.data)) // #nosec G203
			msg := nextOutboundMsg(t, rq)
			if msg.What != what.Inner {
				t.Fatalf("unexpected message type %v", msg.What)
			}
			// Wire data for a string-id message is "id\t<json-quoted-data>".
			_, jsonPart, ok := strings.Cut(msg.Data, "\t")
			if !ok {
				t.Fatalf("expected id\\tdata, got %q", msg.Data)
			}
			var got string
			if err := json.Unmarshal([]byte(jsonPart), &got); err != nil {
				t.Fatalf("data not JSON-parseable (%v): %q", err, jsonPart)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("round-trip mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestRequest_JsCallProducesJawsJSFrameSafeWireData(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tagValue := &testUi{}
	rq.Register(tagValue)

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
			rq.Jaws.JsCall(tagValue, "fn", tt.jsonstr)
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

	tagValue := &testUi{}
	rq.Register(tagValue)

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
			rq.Jaws.JsCall(tagValue, tt.jsfunc, `{"a":1}`)
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
	synctest.Test(t, func(t *testing.T) {
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		elem := rq.NewElement(&testUi{})

		// handleRemove only acts when the container Jid is > 0, so a Remove with a
		// zero Jid must be ignored. synctest.Wait blocks until the process loop has
		// handled the message, so the survival assertion is not vacuous.
		rq.InCh <- wire.WsMsg{What: what.Remove, Jid: 0, Data: elem.Jid().String()}
		synctest.Wait()
		if got := rq.GetElementByJid(elem.Jid()); got == nil {
			t.Fatalf("element %s should not be deletable through zero-container Remove", elem.Jid())
		}
	})
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
	return newTestServerWithSession(true)
}

func newTestServerNoSession() (ts *testServer) {
	return newTestServerWithSession(false)
}

func newTestServerWithSession(withSession bool) (ts *testServer) {
	jw, _ := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	var sess *Session
	if withSession {
		sess = jw.NewSession(rr, hr)
	}
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

func waitForConnectSession(t *testing.T, ch <-chan *Session) (sess *Session) {
	t.Helper()
	select {
	case sess = <-ch:
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for websocket connect")
	}
	return
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

func TestWS_UnclaimedRequestIsGone(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	w := httptest.NewRecorder()
	hr := httptest.NewRequest("", "/", nil)
	rq := jw.NewRequest(hr)
	defer jw.recycle(rq)
	// UseRequest is deliberately not called, so the Request stays unclaimed and
	// startServe() returns false; ServeHTTP must surface an explicit error
	// status instead of an empty 200 OK.
	req := httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	rq.ServeHTTP(w, req)
	if w.Code != http.StatusGone {
		t.Errorf("got %d, want %d", w.Code, http.StatusGone)
	}
}

func TestWS_RejectsMissingOrigin(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("expected handshake to be rejected")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
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
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("expected handshake to be rejected")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestWS_AutoSessionDefaultDoesNotCreateSession(t *testing.T) {
	ts := newTestServerNoSession()
	defer ts.Close()

	sessCh := make(chan *Session, 1)
	ts.rq.SetConnectFn(func(rq *Request) error {
		sessCh <- rq.Session()
		return nil
	})

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if conn != nil {
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}
	if sess := waitForConnectSession(t, sessCh); sess != nil {
		t.Fatalf("expected no session, got %v", sess)
	}
	if cookies := resp.Cookies(); len(cookies) != 0 {
		t.Fatalf("expected no cookies, got %v", cookies)
	}
	if got := ts.jw.SessionCount(); got != 0 {
		t.Fatalf("expected no sessions, got %d", got)
	}
}

func TestWS_AutoSessionCreatesSession(t *testing.T) {
	ts := newTestServerNoSession()
	defer ts.Close()
	ts.jw.AutoSession = true

	sessCh := make(chan *Session, 1)
	ts.rq.SetConnectFn(func(rq *Request) error {
		sessCh <- rq.Session()
		return nil
	})

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if conn != nil {
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}

	sess := waitForConnectSession(t, sessCh)
	if sess == nil {
		t.Fatal("expected session")
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %v", cookies)
	}
	if cookies[0].Name != ts.jw.CookieName {
		t.Fatalf("cookie name %q, want %q", cookies[0].Name, ts.jw.CookieName)
	}
	if cookies[0].Value != sess.CookieValue() {
		t.Fatalf("cookie value %q, want %q", cookies[0].Value, sess.CookieValue())
	}

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	hr.RemoteAddr = sess.IP().String()
	hr.AddCookie(cookies[0])
	if got := ts.jw.GetSession(hr); got != sess {
		t.Fatalf("GetSession() = %v, want %v", got, sess)
	}
}

func TestWS_AutoSessionKeepsExistingSession(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	ts.jw.AutoSession = true

	sessCh := make(chan *Session, 1)
	ts.rq.SetConnectFn(func(rq *Request) error {
		sessCh <- rq.Session()
		return nil
	})

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if conn != nil {
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}
	if sess := waitForConnectSession(t, sessCh); sess != ts.sess {
		t.Fatalf("Session() = %v, want %v", sess, ts.sess)
	}
	if cookies := resp.Cookies(); len(cookies) != 0 {
		t.Fatalf("expected no new cookies, got %v", cookies)
	}
	requests := ts.sess.Requests()
	if len(requests) != 1 || requests[0] != ts.rq {
		t.Fatalf("expected one attached request, got %v", requests)
	}
}

func TestWS_AutoSessionRejectDoesNotCreateSession(t *testing.T) {
	ts := newTestServerNoSession()
	defer ts.Close()
	ts.jw.AutoSession = true

	hdr := http.Header{}
	hdr.Set("Origin", "https://evil.invalid")
	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), &websocket.DialOptions{HTTPHeader: hdr})
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("expected handshake to be rejected")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status %d", resp.StatusCode)
	}
	if cookies := resp.Cookies(); len(cookies) != 0 {
		t.Fatalf("expected no cookies, got %v", cookies)
	}
	if got := ts.jw.SessionCount(); got != 0 {
		t.Fatalf("expected no sessions, got %d", got)
	}
	if sess := ts.rq.Session(); sess != nil {
		t.Fatalf("expected request session to remain nil, got %v", sess)
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
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
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
	testRequestWriter{rq: ts.rq, Writer: httptest.NewRecorder()}.Register(fooItem, func(elem *Element, value string) error {
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
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

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

func TestWS_PingDisconnectsUnresponsiveClient(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	ts.jw.WebSocketPingInterval = 20 * time.Millisecond
	ts.jw.webSocketTimeout = 10 * time.Millisecond

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}

	select {
	case <-ts.connectedCh:
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for websocket connect")
	}

	waitForRequestCount(t, ts.jw, 0, testTimeout)
}

func TestWS_PingDisabledKeepsIdleConnection(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	ts.jw.WebSocketPingInterval = 0

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}

	select {
	case <-ts.connectedCh:
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for websocket connect")
	}

	// This test drives a real WebSocket connection (real network I/O), so it runs
	// on the real clock and cannot use a synctest bubble. Give the connection a
	// moment to settle, then confirm the request counts are stable.
	time.Sleep(150 * time.Millisecond)
	total, active := ts.jw.RequestCounts()
	if total != 1 || active != 1 {
		t.Fatalf("RequestCounts() = %d, %d, want 1, 1", total, active)
	}
	if got := ts.jw.RequestCount(); got != total {
		t.Fatalf("RequestCount() = %d, want %d", got, total)
	}

	_ = conn.CloseNow()
	waitForRequestCounts(t, ts.jw, 0, 0, testTimeout)
}

// waitForRequestCount polls a Jaws served over a real WebSocket connection, so
// it runs on the real clock (deadline-bounded) rather than in a synctest bubble.
func waitForRequestCount(t *testing.T, jw *Jaws, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if got := jw.RequestCount(); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("RequestCount() = %d, want %d", jw.RequestCount(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForRequestCounts(t *testing.T, jw *Jaws, wantTotal, wantActive int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		total, active := jw.RequestCounts()
		if total == wantTotal && active == wantActive {
			return
		}
		if time.Now().After(deadline) {
			total, active = jw.RequestCounts()
			t.Fatalf("RequestCounts() = %d, %d, want %d, %d", total, active, wantTotal, wantActive)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
