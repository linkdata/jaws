package jaws

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

const testTimeout = time.Second * 3

type eventErrorLogger struct {
	mu   sync.Mutex
	errs []error
}

func (*eventErrorLogger) Info(string, ...any) {}
func (*eventErrorLogger) Warn(string, ...any) {}

func (l *eventErrorLogger) Error(_ string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			if err, ok := args[i+1].(error); ok {
				l.errs = append(l.errs, err)
			}
		}
	}
}

func (l *eventErrorLogger) loggedErrors() (errs []error) {
	l.mu.Lock()
	errs = append(errs, l.errs...)
	l.mu.Unlock()
	return
}

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

func TestRequest_DeleteElementNil(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	// GetElementByJid returns nil for an unknown Jid, so forwarding that result to
	// DeleteElement must be a no-op rather than a nil dereference, matching the rest of
	// the nil-tolerant *Element-accepting Request methods (Tag, TagExpanded, TagsOf).
	rq.DeleteElement(rq.GetElementByJid(Jid(999)))
	rq.DeleteElement(nil)
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
	is.Equal(rq.wantMessage(&wire.Message{Dest: "Jid.1"}), false)
}

func TestRequest_wantMessage_KeyDest(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	liveKey := rq.JawsKey
	is.True(liveKey != 0)

	tests := []struct {
		name string
		dest key.Key
		want bool
	}{
		{"matching key", liveKey, true},
		{"other key", liveKey + 1, false},
		{"zero key", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTestHelper(t).Equal(rq.wantMessage(&wire.Message{Dest: tt.dest}), tt.want)
		})
	}
}

// TestRequest_wantMessage_RejectsRecycledKey is the core broadcast regression:
// matching on the identity key rather than the *Request pointer lets the loop
// reject a message aimed at a request that was recycled and reused under a new key,
// even when it is the very same pooled object. A dest==*Request match could not
// tell the reused object from the original; a key match can.
func TestRequest_wantMessage_RejectsRecycledKey(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	staleKey := rq.JawsKey
	is.True(rq.wantMessage(&wire.Message{Dest: staleKey}))

	// Recycle zeroes the key, so the same object no longer matches the old key.
	jw.recycle(rq)
	is.Equal(rq.wantMessage(&wire.Message{Dest: staleKey}), false)

	// Reuse the same object under a fresh key, as the pool handing it back to a new
	// connection would. Re-key directly under rq.mu (the lock destKey/wantMessage
	// use) so the aliasing is deterministic rather than dependent on the pool
	// returning this struct. A message still aimed at the old key must be rejected
	// even though rq is the same object it once identified; one aimed at the new
	// key is accepted.
	newKey := staleKey + 1
	rq.mu.Lock()
	rq.JawsKey = newKey
	rq.mu.Unlock()
	is.Equal(rq.wantMessage(&wire.Message{Dest: staleKey}), false)
	is.True(rq.wantMessage(&wire.Message{Dest: newKey}))
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
	b, sent := rq.drainTailScript()
	if err := rq.writeTailResponse(w, b, sent); err != nil {
		t.Fatal(err)
	}
	s := w.Body.String()
	if strings.Contains(s, "</script><img") {
		t.Fatalf("writeTailScript did not escape </script> in attribute value: %s", s)
	}
	th.True(strings.Contains(s, `\x3c/script>`))
}

func TestRequest_writeTailScript_QuotesAstralAndLineSeparators(t *testing.T) {
	th := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)
	item := &testUi{}
	e := rq.NewElement(item)
	// U+1FFFE is a non-printable astral code point: strconv.Quote would emit it as the
	// Go-only escape \U0001fffe, which JavaScript silently mis-decodes to the literal
	// text "U0001fffe", so the value must instead survive as literal UTF-8. U+2028 is a
	// JavaScript line separator that must be escaped so it cannot break the inline
	// <script> string literal.
	e.SetAttr("data-x", "a\U0001FFFEb\u2028c")

	w := httptest.NewRecorder()
	b, sent := rq.drainTailScript()
	if err := rq.writeTailResponse(w, b, sent); err != nil {
		t.Fatal(err)
	}
	s := w.Body.String()
	// No Go-only \U escape: JavaScript drops the backslash and keeps the letters.
	if strings.Contains(s, `\U`) {
		t.Fatalf("tail script contains a Go-only \\U escape JavaScript cannot decode: %s", s)
	}
	// The astral rune survives verbatim as literal UTF-8.
	th.True(strings.Contains(s, "\U0001FFFE"))
	// The line separator is escaped, not emitted literally.
	th.True(strings.Contains(s, `\u2028`))
	if strings.ContainsRune(s, '\u2028') {
		t.Fatalf("tail script contains a literal U+2028 line separator: %q", s)
	}
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
	b, sent := rq.drainTailScript()
	if err := rq.writeTailResponse(w, b, sent); err != nil {
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
	b, sent := rq.drainTailScript()
	if err := rq.writeTailResponse(w, b, sent); err != nil {
		t.Fatal(err)
	}
	s := w.Body.String()
	th.True(strings.Contains(s, `removeAttribute("hidden");`))
	th.True(strings.Contains(s, `classList?.remove("cls");`))

	rq.muQueue.Lock()
	th.Equal(len(rq.wsQueue), 0)
	rq.muQueue.Unlock()
}

// TestRequest_writeTailScript_IsolatesEachFixup verifies each attribute/class fixup
// is wrapped in its own try/catch, so a fixup that throws at runtime (e.g. a class
// token containing whitespace, which the ?. element guard does not catch) cannot
// abandon the fixups that follow it. The drain removes these messages from wsQueue,
// making the tail script their sole applier, so the isolation mirrors the per-order
// isolation the WebSocket client applies in jawsMessage.
func TestRequest_writeTailScript_IsolatesEachFixup(t *testing.T) {
	th := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)
	e1 := rq.NewElement(&testUi{})
	e2 := rq.NewElement(&testUi{})
	e3 := rq.NewElement(&testUi{})
	// A valid fixup, then one whose class token throws in the browser (whitespace is
	// not a valid classList token), then another valid fixup.
	e1.SetClass("ok-first")
	e2.SetClass("btn primary")
	e3.SetClass("ok-last")

	w := httptest.NewRecorder()
	b, sent := rq.drainTailScript()
	if err := rq.writeTailResponse(w, b, sent); err != nil {
		t.Fatal(err)
	}
	s := w.Body.String()
	// One try and one catch per fixup.
	th.Equal(strings.Count(s, "try{document.getElementById("), 3)
	th.Equal(strings.Count(s, "}catch(e){console.error(e);}"), 3)
	// The fixup after the throwing one lives in its own isolated statement, so the
	// throwing one cannot prevent it from running.
	th.True(strings.Contains(s, `classList?.add("ok-last");}catch(e){console.error(e);}`))
}

// TestRequest_TailScriptConcurrentWithRecycle exercises a /jaws/.tail fetch
// racing recycle of the same still-pending request. The handler holds jw.mu (read)
// across the drainTailScript call and recycle needs the jw.mu write lock, so the
// drain and recycle are serialized: the fetch can never drain a recycled+reused
// request. clearLocked also takes muQueue to reset wsQueue/tailsent, the lock
// drainTailScript holds, so that reset cannot race the drain either. Run with -race.
func TestRequest_TailScriptConcurrentWithRecycle(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	// Fetch the tail script via the public endpoint while recycling the same
	// request, repeated enough to overlap under the race detector.
	const n = 300
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		rq := jw.NewRequest(nil)
		e := rq.NewElement(&testUi{})
		e.SetAttr("hidden", "yes")
		e.SetClass("cls")
		tailURL := "/jaws/.tail/" + rq.JawsKeyString()
		wg.Add(2)
		go func() {
			defer wg.Done()
			jw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tailURL, nil))
		}()
		go func() {
			defer wg.Done()
			jw.recycle(rq)
		}()
	}
	wg.Wait()
}

// TestRequest_wantMessageConcurrentWithRecycle stresses the broadcast identity
// check: wantMessage reads rq.JawsKey under rq.mu while recycle zeroes it under the
// same lock. The read and write must be serialized so there is no data race on the
// key. Run with -race.
func TestRequest_wantMessageConcurrentWithRecycle(t *testing.T) {
	jw, _ := New()
	defer jw.Close()

	const n = 300
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		rq := jw.NewRequest(nil)
		staleKey := rq.JawsKey
		wg.Add(2)
		go func() {
			defer wg.Done()
			rq.wantMessage(&wire.Message{Dest: staleKey})
		}()
		go func() {
			defer wg.Done()
			jw.recycle(rq)
		}()
	}
	wg.Wait()
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

type deferredAfterContext struct {
	context.Context
	mu       sync.Mutex
	done     chan struct{}
	err      error
	callback func()
	onAfter  func()
}

func (ctx *deferredAfterContext) Done() <-chan struct{} { return ctx.done }

func (ctx *deferredAfterContext) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	return ctx.err
}

func (ctx *deferredAfterContext) AfterFunc(fn func()) func() bool {
	if ctx.onAfter != nil {
		ctx.onAfter()
	}
	ctx.mu.Lock()
	ctx.callback = fn
	ctx.mu.Unlock()
	return func() bool { return false }
}

func TestRequest_SetContextRegistersAfterFuncOutsideLock(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	rq := jw.NewRequest(nil)
	observed := make(chan context.Context, 1)
	custom := &deferredAfterContext{
		Context: rq.Context(),
		done:    make(chan struct{}),
		onAfter: func() {
			observed <- rq.Context()
		},
	}
	setDone := make(chan struct{})
	go func() {
		rq.SetContext(func(context.Context) context.Context { return custom })
		close(setDone)
	}()

	select {
	case <-setDone:
	case <-time.After(2 * time.Second):
		t.Fatal("SetContext deadlocked while a custom AfterFunc hook re-entered Request.Context")
	}
	select {
	case got := <-observed:
		if got != custom {
			t.Fatalf("AfterFunc hook observed context %T, want replacement context", got)
		}
	default:
		t.Fatal("replacement context's AfterFunc hook was not registered")
	}
	jw.Close()
}

func (ctx *deferredAfterContext) cancel() {
	ctx.mu.Lock()
	ctx.err = context.Canceled
	close(ctx.done)
	ctx.mu.Unlock()
}

func (ctx *deferredAfterContext) fire() {
	ctx.mu.Lock()
	fn := ctx.callback
	ctx.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func TestRequest_SetContextDelayedCallbackDoesNotCancelReusedRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	first := jw.NewRequest(nil)
	callbackCalled := make(chan struct{}, 1)
	var callbackArmed atomic.Bool
	first.mu.Lock()
	firstCancel := first.cancelFn
	first.cancelFn = func(cause error) {
		firstCancel(cause)
		if callbackArmed.Load() {
			callbackCalled <- struct{}{}
		}
	}
	first.mu.Unlock()
	deferred := &deferredAfterContext{done: make(chan struct{})}
	first.SetContext(func(old context.Context) context.Context {
		deferred.Context = old
		return deferred
	})
	deferred.cancel()
	jw.recycle(first)

	poolNew := jw.reqPool.New
	jw.reqPool.New = func() any { return first }
	defer func() { jw.reqPool.New = poolNew }()
	second := jw.NewRequest(nil)
	if second != first {
		t.Fatal("request pool did not reuse the Request")
	}
	secondCtx := second.Context()
	callbackArmed.Store(true)
	deferred.fire()
	select {
	case <-callbackCalled:
	case <-time.After(testTimeout):
		t.Fatal("delayed SetContext callback did not run")
	}
	select {
	case <-secondCtx.Done():
		t.Fatal("delayed SetContext callback canceled the reused Request")
	default:
	}
	jw.recycle(second)
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
	rq.lastWriteSeconds.Store(jw.runtimeSeconds.Load() - 3600)

	if jw.UseRequest(rq.JawsKey, hr) != rq {
		t.Fatal("expected claim to succeed")
	}
	// claim refreshed the write second, so the just-claimed request is not idle.
	if expired, _ := rq.maintenance(jw.runtimeSeconds.Load(), time.Second); expired {
		t.Fatal("a freshly claimed request must not be treated as idle by maintenance")
	}

	// If a request is recycled anyway, startServe must refuse to drive it.
	jw.recycle(rq)
	if rq.startServe() {
		t.Fatal("startServe must refuse a recycled request")
	}
}

func TestRequest_AdvanceLastWriteSeconds(t *testing.T) {
	tests := []struct {
		name     string
		previous int32
		now      int32
		want     int32
	}{
		{name: "forward", previous: 1, now: 2, want: 2},
		{name: "backward", previous: 2, now: 1, want: 2},
		{name: "equal", previous: 2, now: 2, want: 2},
		{name: "wrap forward", previous: 1<<31 - 1, now: -1 << 31, want: -1 << 31},
		{name: "wrap backward", previous: -1 << 31, now: 1<<31 - 1, want: -1 << 31},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq := &Request{}
			rq.lastWriteSeconds.Store(tt.previous)
			rq.advanceLastWriteSeconds(tt.now)
			if got := rq.lastWriteSeconds.Load(); got != tt.want {
				t.Fatalf("lastWriteSeconds = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRequest_ClaimRejectsCanceledRequest(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	wantCause := errors.New("cancel before claim")
	rq.Cancel(wantCause)

	err = rq.claim(hr)
	if !errors.Is(err, ErrRequestCancelled) {
		t.Fatalf("claim error = %v, want ErrRequestCancelled", err)
	}
	if !errors.Is(err, wantCause) {
		t.Fatalf("claim error = %v, want cause %v", err, wantCause)
	}
	if rq.claimed.Load() {
		t.Fatal("canceled request was marked claimed")
	}
	if rq.httpDoneCh != nil {
		t.Fatal("canceled request retained the claiming HTTP context")
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
			rq.Jaws.Broadcast(wire.Message{Dest: rq.JawsKey})
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

func TestBroadcast_ZeroKeyDestDropped(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq := tj.newRequest(nil)

	// A zero key (captured from an already-recycled request) targets no live
	// request: Broadcast drops it before it reaches the Serve loop rather than
	// treating it as a tag. A real follow-up still arrives, proving only the
	// zero-key message was dropped, and no bad-destination misuse is logged.
	rq.Jaws.Broadcast(wire.Message{Dest: key.Key(0), What: what.Alert, Data: alertData("info", "dropped")})
	rq.Alert("info", "kept")

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.OutCh:
		s := msg.Format()
		th.True(strings.Contains(s, "kept"))
		th.Equal(strings.Contains(s, "dropped"), false)
	}
	select {
	case s := <-rq.OutCh:
		t.Errorf("unexpected second delivery: %q", s)
	default:
	}
	th.Equal(strings.Contains(tj.log.String(), "jaws: Broadcast"), false)
}

// TestRequest_ProducersSkipRecycled covers the destKey()==0 branch in Request.Alert
// and Request.Redirect. Once a Request is recycled it carries the zero key, so both
// take the skip branch and never call Broadcast. A zero-key broadcast would be
// dropped by Jaws.Broadcast anyway (see TestBroadcast_ZeroKeyDestDropped); the guard
// avoids the round-trip, and exercising it is the only coverage of the branch.
func TestRequest_ProducersSkipRecycled(t *testing.T) {
	th := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	jw.recycle(rq)
	th.Equal(rq.destKey(), key.Key(0))

	// No Serve loop drains bcastCh, so any stray broadcast would be observable.
	rq.Alert("info", "ignored")
	rq.Redirect("/ignored")

	select {
	case msg := <-jw.bcastCh:
		t.Fatalf("recycled request must not broadcast, got %#v", msg)
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
	da := &DefaultAuth{logger: slog.New(slog.NewTextHandler(&buf, nil))}
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

func TestRequest_ElemsStayAscendingAfterDeletes(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	const n = 50
	elems := make([]*Element, n)
	for i := range elems {
		elems[i] = rq.NewElement(&testUi{})
	}

	// Delete an arbitrary, non-contiguous subset.
	deleted := map[Jid]bool{}
	for i, e := range elems {
		if i%3 == 0 || i%7 == 0 {
			rq.DeleteElement(e)
			deleted[e.Jid()] = true
		}
	}

	// (a) GetElementByJid resolves every surviving jid and returns nil for deleted ones.
	for _, e := range elems {
		got := rq.GetElementByJid(e.Jid())
		if deleted[e.Jid()] {
			if got != nil {
				t.Fatalf("jid %s was deleted but GetElementByJid returned %+v", e.Jid(), got)
			}
			continue
		}
		if got != e {
			t.Fatalf("jid %s: GetElementByJid = %+v, want %+v", e.Jid(), got, e)
		}
	}

	// (b) the surviving rq.elems jids are strictly ascending.
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	for i := 1; i < len(rq.elems); i++ {
		if prev, cur := rq.elems[i-1].Jid(), rq.elems[i].Jid(); prev >= cur {
			t.Fatalf("rq.elems not strictly ascending at index %d: %s >= %s", i, prev, cur)
		}
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

	elem1 := rq1.NewElement(testDivWidget{inner: "one"})
	elem1.AddHandlers(tjc1)
	elem1.Freeze()
	elem2 := rq2.NewElement(testDivWidget{inner: "two"})
	elem2.AddHandlers(tjc2)
	elem2.Freeze()

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
		{
			// An origin like "http:///" parses to an empty Host; the uhost != ""
			// guard must reject it rather than let an empty origin host match an
			// (also empty) initial host.
			name:       "empty origin host rejected",
			initialURL: "http://example.test/page",
			origin:     "http:///",
			wantErr:    ErrWebsocketOriginWrongHost,
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

func TestNormalizedWebSocketAcceptRequest(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		origin     string
		wantHost   string
		wantOrigin string
		wantClone  bool
	}{
		{
			name:       "HTTP Host default port removed",
			host:       "example.test:80",
			origin:     "http://example.test",
			wantHost:   "example.test",
			wantOrigin: "http://example.test",
			wantClone:  true,
		},
		{
			name:       "HTTP Origin default port removed",
			host:       "example.test",
			origin:     "http://example.test:80",
			wantHost:   "example.test",
			wantOrigin: "http://example.test",
			wantClone:  true,
		},
		{
			name:       "HTTPS IPv6 default port removed",
			host:       "[2001:db8::1]:443",
			origin:     "https://[2001:db8::1]:443",
			wantHost:   "[2001:db8::1]",
			wantOrigin: "https://[2001:db8::1]",
			wantClone:  true,
		},
		{
			name:       "non-default port preserved",
			host:       "example.test:8080",
			origin:     "http://example.test:8080",
			wantHost:   "example.test:8080",
			wantOrigin: "http://example.test:8080",
			wantClone:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/jaws/key", nil)
			r.Host = tt.host
			r.Header.Set("Origin", tt.origin)

			normalized := normalizedWebSocketAcceptRequest(r)
			if normalized.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", normalized.Host, tt.wantHost)
			}
			if origin := normalized.Header.Get("Origin"); origin != tt.wantOrigin {
				t.Errorf("Origin = %q, want %q", origin, tt.wantOrigin)
			}
			if cloned := normalized != r; cloned != tt.wantClone {
				t.Errorf("cloned = %t, want %t", cloned, tt.wantClone)
			}
			if r.Host != tt.host || r.Header.Get("Origin") != tt.origin {
				t.Errorf("original request mutated: Host = %q, Origin = %q", r.Host, r.Header.Get("Origin"))
			}
		})
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
		n1 := atomic.LoadInt32(&tss1.getCalled)
		n2 := atomic.LoadInt32(&tss2.getCalled)
		// The lower bound is intentional: synctest scheduling can collapse or split
		// the two Dirty calls across one or two 1ms updateTicker fires, so the exact
		// JawsGet count is 2 or 3 and an == assertion would be flaky.
		th.True(n1 >= 2)
		th.True(n2 >= 2)
		// Pin an upper bound by proving the system quiesces: with jw.dirty now empty,
		// distributeDirt returns 0, no further what.Update is broadcast, so getCalled
		// must not increase. This catches a runaway re-render/re-broadcast regression.
		time.Sleep(2 * time.Millisecond)
		synctest.Wait()
		th.Equal(atomic.LoadInt32(&tss1.getCalled), n1)
		th.Equal(atomic.LoadInt32(&tss2.getCalled), n2)
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
		},
	}
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
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		container := rq.NewElement(&testUi{})
		child := rq.NewElement(&testUi{})

		// Send the incoming Remove and let the process loop handle it; the
		// element is gone once every bubbled goroutine is durably blocked again.
		rq.InCh <- wire.WsMsg{What: what.Remove, Jid: container.Jid(), Data: child.Jid().String()}
		synctest.Wait()
		if got := rq.GetElementByJid(child.Jid()); got != nil {
			t.Fatalf("child element %s should be removed, got %+v", child.Jid(), got)
		}
		if got := rq.GetElementByJid(container.Jid()); got == nil {
			t.Fatalf("container element %s should still exist", container.Jid())
		}
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

	// With an initial request the message carries its method and URI.
	if !errors.As(err, &target2) {
		t.Fatal("expected err to be an errRequestCancelled")
	}
	wantWithInitial := fmt.Sprintf("Request<%s>: %s %q: %v",
		target2.JawsKey, target2.Method, target2.RequestURI, cause)
	th.Equal(target2.Error(), wantWithInitial)

	// Without an initial request the method/URI fragment is omitted entirely.
	noInitial := errRequestCancelled{JawsKey: target2.JawsKey, Cause: cause}
	th.Equal(noInitial.Error(), fmt.Sprintf("Request<%s>: %v", target2.JawsKey, cause))
}

// TestRequest_getSendMsgsKeepsOrderFromDeletedElement verifies that a page-global
// what.Order command survives the getSendMsgs drain even when the element that
// issued it has been deleted: the browser ignores the Jid for Order, and the
// referenced elements may still be present.
func TestRequest_getSendMsgsKeepsOrderFromDeletedElement(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	childA := rq.NewElement(&testUi{})
	childB := rq.NewElement(&testUi{})
	ctrl := rq.NewElement(&testUi{})

	// The control reorders two live siblings, then removes itself in the same pass.
	ctrl.Order([]jid.Jid{childA.Jid(), childB.Jid()})
	rq.DeleteElement(ctrl)

	var sawOrder bool
	for _, m := range rq.getSendMsgs() {
		if m.What == what.Order {
			sawOrder = true
		}
	}
	if !sawOrder {
		t.Fatal("page-global Order referencing live children was dropped because the issuing element was deleted")
	}
}

// TestRequest_getSendMsgsDropsCallFromDeletedElement verifies the complementary
// case: an element-targeted what.Call must still be dropped when its element is
// gone, since the browser resolves it via getElementById and would otherwise throw.
func TestRequest_getSendMsgsDropsCallFromDeletedElement(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	elem := rq.NewElement(&testUi{})
	elem.JsCall("fn", "{}")
	rq.DeleteElement(elem)

	for _, m := range rq.getSendMsgs() {
		if m.What == what.Call {
			t.Fatal("element-targeted Call for a deleted element must be dropped")
		}
	}
}

// TestRequest_queueEventOverloadCancels verifies that when the event-call channel is
// full, the Request is cancelled with a cause that wraps ErrRequestOverloaded (and is
// still matchable as ErrRequestCancelled). It drives queueEvent with an unbuffered
// channel that has no receiver, so the non-blocking send fails immediately.
func TestRequest_queueEventOverloadCancels(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	full := make(chan eventFnCall) // unbuffered, no receiver: send fails at once
	rq.queueEvent(full, eventFnCall{jid: 1, wht: what.Input, data: "x"})

	cause := context.Cause(rq.Context())
	if !errors.Is(cause, ErrRequestOverloaded) {
		t.Fatalf("cause = %v, want it to wrap ErrRequestOverloaded", cause)
	}
	if !errors.Is(cause, ErrRequestCancelled) {
		t.Fatalf("cause = %v, want it to wrap ErrRequestCancelled", cause)
	}
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
	if err := e.renderDebug(&sb); err != nil {
		t.Fatal(err)
	}

	txt := sb.String()
	is.Equal(strings.Contains(txt, "zomg"), true)
	is.Equal(strings.Contains(txt, "n/a"), false)

	rq.mu.Lock()
	defer rq.mu.Unlock()
	sb.Reset()
	if err := e.renderDebug(&sb); err != nil {
		t.Fatal(err)
	}

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

	subDone := make(chan chan wire.Message)
	go func() {
		subDone <- jw.subscribe(rq, 1)
	}()
	sub := <-jw.subCh
	msgCh := <-subDone
	if msgCh == nil {
		t.Fatal("expected non-nil subscription channel")
	}
	if sub.msgCh != msgCh {
		t.Fatal("unexpected subscription")
	}
	unsubDone := make(chan struct{})
	go func() {
		jw.unsubscribe(msgCh)
		close(unsubDone)
	}()
	if got := <-jw.unsubCh; got != msgCh {
		t.Fatal("unexpected unsubscribe channel")
	}
	<-unsubDone

	// Request timeout path.
	rq.lastWriteSeconds.Store(jw.runtimeSeconds.Load() - 3600)
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
	if err := rqA.claim(hrB); !errors.Is(err, ErrWebSocketIPMismatch) {
		t.Fatalf("expected ErrWebSocketIPMismatch, got %v", err)
	} else if !strings.Contains(err.Error(), `expected IP "1.2.3.4", got "2.2.2.2"`) {
		t.Fatalf("unexpected error text: %v", err)
	}

	nowSeconds := jw.runtimeSeconds.Load()
	rqM := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	rqM.lastWriteSeconds.Store(nowSeconds - 3600)
	if expired, _ := rqM.maintenance(nowSeconds, time.Second); !expired {
		t.Fatal("expected maintenance timeout")
	}
	rqR := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	rqR.MarkWritten()
	if expired, _ := rqR.maintenance(jw.runtimeSeconds.Load(), time.Hour); expired {
		t.Fatal("a freshly written request must not be idle-expired")
	}
	rqC := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	rqC.cancel(errors.New("cancelled"))
	if expired, _ := rqC.maintenance(jw.runtimeSeconds.Load(), time.Hour); !expired {
		t.Fatal("expected maintenance cancellation")
	}
	rqOK := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	rqOK.MarkWritten()
	if expired, _ := rqOK.maintenance(jw.runtimeSeconds.Load(), time.Hour); expired {
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

func TestNewRequest_SeedsLastWriteFromLiveElapsed(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	// Simulate an hour of startup before Serve runs. Serve is deliberately not
	// started, so runtimeSeconds is never advanced by its maintenance loop.
	jw.created = time.Now().Add(-time.Hour)

	rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))

	// The seed must reflect true elapsed time (~3600s), not the stale zero that
	// runtimeSeconds still holds before Serve seeds it.
	if got := rq.lastWriteSeconds.Load(); got < 3595 || got > 3605 {
		t.Fatalf("expected lastWriteSeconds seeded to ~3600, got %d", got)
	}

	// A brand-new request must not be idle-expired by the first maintenance pass.
	if expired, _ := rq.maintenance(jw.runtimeSeconds.Load(), time.Hour); expired {
		t.Fatal("a brand-new pre-Serve request must not be idle-expired")
	}
}

func TestCoverage_RequestProcessHTTPDoneAndBroadcastDone(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	ctx, cancel := context.WithCancel(t.Context())
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
	unsubDone := make(chan chan wire.Message, 1)
	go func() {
		unsubDone <- <-jw.unsubCh
	}()
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
	if got := <-unsubDone; got != bcastCh {
		t.Fatalf("unexpected unsubscribe channel %p, want %p", got, bcastCh)
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

func TestRequest_Template_Unwrapped(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	if err := rq.Template("", "testtemplate", tag.Tag("dot")); err != nil {
		t.Fatal(err)
	}
	is.Equal(len(rq.elems), 1)
	if got := rq.BodyHTML(); !strings.Contains(string(got), "dot") {
		t.Errorf("Request.Template() = %q, want it to contain %q", got, "dot")
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

	// NewElement asserts runtime comparability in debug builds. The all-build guard
	// against unusable UI values lives in the container widgets (see the ui package),
	// since that is the only place a raw UI value is used as a map key.
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for non-comparable UI when comparable check is enabled")
		}
	}()
	rq.NewElement(testUnhashableUI{m: map[string]int{"x": 1}})
}

func TestRequest_getElementByJidLocked_DebugUnsortedPanics(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("debug checks not enabled")
	}

	// rq.elems must stay sorted ascending by Jid for the binary search; a debug
	// build asserts it rather than silently returning wrong lookups.
	rq := &Request{elems: []*Element{{jid: 2}, {jid: 1}}}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when rq.elems is not sorted by Jid")
		}
	}()
	rq.getElementByJidLocked(1)
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

func TestRequest_IncomingRemoveDoesNotDeleteMessageJidListedInData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		elem := rq.NewElement(&testUi{})

		// Data is supposed to contain removed child IDs. If a malformed client
		// names the container itself, the cleanup acknowledgement must still leave
		// the container registered.
		rq.InCh <- wire.WsMsg{What: what.Remove, Jid: elem.Jid(), Data: elem.Jid().String()}
		synctest.Wait()
		if got := rq.GetElementByJid(elem.Jid()); got == nil {
			t.Fatalf("element %s should still exist after Remove data names the container", elem.Jid())
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
		{
			// Raw control bytes inside a string value make json.Compact fail;
			// they must be escaped, not passed through, or they break framing.
			name:    "raw tab inside string value",
			jsonstr: "{\"a\":\"x\ty\"}",
		},
		{
			name:    "raw newline inside string value",
			jsonstr: "{\"a\":\"x\ny\"}",
		},
		{
			name:    "raw carriage return inside string value",
			jsonstr: "{\"a\":\"x\ry\"}",
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
			if strings.Contains(wire, "\r") {
				t.Fatalf("wire message contains embedded carriage return: %q", wire)
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
		{
			name:   "carriage return in function path",
			jsfunc: "fn\rpart",
		},
		{
			name:   "equals in function path",
			jsfunc: "fn=part",
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
			if strings.Contains(wire, "\r") {
				t.Fatalf("wire message contains embedded carriage return: %q", wire)
			}
			if msg.Data != `fnpart={"a":1}` {
				t.Fatalf("Call payload = %q, want %q", msg.Data, `fnpart={"a":1}`)
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

type observedDoneContext struct {
	context.Context
	armed    atomic.Bool
	once     sync.Once
	observed chan struct{}
}

func (ctx *observedDoneContext) Done() <-chan struct{} {
	if ctx.armed.Load() {
		ctx.once.Do(func() { close(ctx.observed) })
	}
	return ctx.Context.Done()
}

func newTestServer(t *testing.T) (ts *testServer) {
	t.Helper()
	return newTestServerWithSession(t, true, nil)
}

func newTestServerWithLogger(t *testing.T, logger Logger) (ts *testServer) {
	t.Helper()
	return newTestServerWithSession(t, true, logger)
}

func newTestServerNoSession(t *testing.T) (ts *testServer) {
	t.Helper()
	return newTestServerWithSession(t, false, nil)
}

func newTestServerWithSession(t *testing.T, withSession bool, logger Logger) (ts *testServer) {
	t.Helper()
	jw, _ := New()
	jw.Logger = logger
	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
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
	go jw.Serve()
	waitForServeLoop(t, jw)
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
		jawsKey, _ := key.Parse(strings.TrimPrefix(r.URL.Path, "/jaws/"))
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

func TestWS_AcceptsSameOriginDefaultPort(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		secure bool
	}{
		{
			name:   "Host has explicit port",
			host:   "example.test:80",
			origin: "http://example.test",
		},
		{
			name:   "Origin has explicit port",
			host:   "example.test",
			origin: "http://example.test:80",
		},
		{
			name:   "HTTPS Host has explicit port",
			host:   "example.test:443",
			origin: "https://example.test",
			secure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			defer ts.Close()

			ts.hr.Host = tt.host
			if tt.secure {
				ts.hr.TLS = &tls.ConnectionState{}
			}
			hdr := http.Header{}
			hdr.Set("Origin", tt.origin)
			conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), &websocket.DialOptions{
				HTTPHeader: hdr,
				Host:       tt.host,
			})
			if err != nil {
				status := 0
				if resp != nil {
					status = resp.StatusCode
				}
				t.Fatalf("same-origin callback rejected: status=%d err=%v", status, err)
			}
			defer func() { _ = conn.CloseNow() }()
			if resp.StatusCode != http.StatusSwitchingProtocols {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
			}
		})
	}
}

func TestWS_RejectsMissingOrigin(t *testing.T) {
	ts := newTestServer(t)
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
	ts := newTestServer(t)
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

func TestWS_RejectsCallbackHostMismatch(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	ts.hr.Host = "example.test"
	hdr := http.Header{}
	hdr.Set("Origin", "http://example.test")
	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), &websocket.DialOptions{
		HTTPHeader: hdr,
		Host:       "other.test:80",
	})
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
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestWS_AutoSessionDefaultDoesNotCreateSession(t *testing.T) {
	ts := newTestServerNoSession(t)
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
	ts := newTestServerNoSession(t)
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
	ts := newTestServer(t)
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
	ts := newTestServerNoSession(t)
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

func TestWS_AutoSessionFailedHandshakeDoesNotCreateSession(t *testing.T) {
	// websocket.Dial always sends a well-formed handshake, so these defects
	// that only websocket.Accept detects must be sent as raw HTTP requests.
	tests := []struct {
		name       string
		hdrs       map[string]string
		wantStatus int
	}{
		{
			name:       "missing Connection and Upgrade headers",
			hdrs:       map[string]string{"Sec-WebSocket-Version": "13"},
			wantStatus: http.StatusUpgradeRequired,
		},
		{
			name: "unsupported Sec-WebSocket-Version",
			hdrs: map[string]string{
				"Connection":            "Upgrade",
				"Upgrade":               "websocket",
				"Sec-WebSocket-Version": "12",
			},
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A Request serves only once even on a failed handshake, so each
			// variant needs its own testServer.
			ts := newTestServerNoSession(t)
			defer ts.Close()
			ts.jw.AutoSession = true

			req, err := http.NewRequestWithContext(ts.ctx, http.MethodGet, ts.Url(), nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Origin", ts.origin())
			req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
			for k, v := range tt.hdrs {
				req.Header.Set(k, v)
			}
			resp, err := ts.srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.wantStatus)
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
		})
	}
}

type writeHeaderNowRecorder struct {
	*httptest.ResponseRecorder
	calledWriteHeaderNow bool
}

func (w *writeHeaderNowRecorder) WriteHeaderNow() { w.calledWriteHeaderNow = true }

func TestAutoSessionWriter_ForwardsWriteHeaderNow(t *testing.T) {
	rec := &writeHeaderNowRecorder{ResponseRecorder: httptest.NewRecorder()}
	asw := &autoSessionWriter{ResponseWriter: rec}
	asw.WriteHeaderNow()
	if !rec.calledWriteHeaderNow {
		t.Fatal("expected WriteHeaderNow to be forwarded to the wrapped writer")
	}
}

func TestWS_ConnectFnFailsWithoutMessage(t *testing.T) {
	const nope = "nope"
	ts := newTestServer(t)
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
	readCtx, cancelRead := context.WithTimeout(t.Context(), testTimeout)
	defer cancelRead()
	if _, _, err = conn.Read(readCtx); err == nil {
		t.Fatal("ConnectFn failure sent a WebSocket message")
	}
	if readCtx.Err() != nil {
		t.Fatal("WebSocket remained open after ConnectFn failure")
	}
}

func TestWS_ConnectFnFailureDoesNotBlockOnNonReadingPeer(t *testing.T) {
	const requestTimeout = 100 * time.Millisecond
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.ServeWithTimeout(requestTimeout)
	waitForServeLoop(t, jw)

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	initial := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rq := jw.NewRequest(initial)
	if got := jw.UseRequest(rq.JawsKey, initial); got != rq {
		t.Fatalf("UseRequest() = %v, want %v", got, rq)
	}
	rqCtx := rq.Context()
	connectErr := errors.New(strings.Repeat("connect rejected ", 1<<19))
	rq.SetConnectFn(func(*Request) error {
		return connectErr
	})

	srv := httptest.NewServer(rq)
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	initial.Host = u.Host
	initial.URL.Scheme = u.Scheme
	initial.URL.Host = u.Host

	hdr := http.Header{}
	hdr.Set("Origin", srv.URL)
	conn, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}

	// Deliberately do not read from conn. Before the failure path closed the
	// socket directly, formatting and writing this large error blocked before
	// cancellation once the peer's receive buffers filled.
	select {
	case <-rqCtx.Done():
	case <-time.After(requestTimeout * 5):
		t.Fatal("ConnectFn failure did not cancel a Request with a non-reading peer")
	}
	if !errors.Is(context.Cause(rqCtx), connectErr) {
		t.Fatal("Request cancellation did not retain the ConnectFn failure")
	}
	waitForRequestCount(t, jw, 0, requestTimeout*5)
}

func TestWS_ConnectFnBroadcastDelivered(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	ts.rq.SetConnectFn(func(rq *Request) error {
		rq.Redirect("/from-connect-fn")
		return nil
	})

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}

	messageType, data, err := conn.Read(ts.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("message type = %v, want %v", messageType, websocket.MessageText)
	}
	msg, ok := wire.Parse(data)
	if !ok {
		t.Fatalf("invalid wire message %q", data)
	}
	if msg.What != what.Redirect || msg.Jid != 0 || msg.Data != "/from-connect-fn" {
		t.Fatalf("message = %+v, want request Redirect to /from-connect-fn", msg)
	}
}

func TestWS_ConnectFnSubscriptionCleanup(t *testing.T) {
	errRejected := errors.New("connect rejected")
	tests := []struct {
		name      string
		connectFn ConnectFn
	}{
		{
			name: "error",
			connectFn: func(*Request) error {
				return errRejected
			},
		},
		{
			name: "panic",
			connectFn: func(*Request) error {
				panic("connect panic")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()

			ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
			defer cancel()
			initial := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			rq := jw.NewRequest(initial)
			if got := jw.UseRequest(rq.JawsKey, initial); got != rq {
				t.Fatalf("UseRequest() = %v, want %v", got, rq)
			}
			rq.SetConnectFn(tt.connectFn)

			type subscriptionPair struct {
				subscribed   chan wire.Message
				unsubscribed chan wire.Message
			}
			cleanupCh := make(chan subscriptionPair, 1)
			go func() {
				select {
				case sub := <-jw.subCh:
					select {
					case msgCh := <-jw.unsubCh:
						close(sub.msgCh)
						cleanupCh <- subscriptionPair{subscribed: sub.msgCh, unsubscribed: msgCh}
					case <-jw.Done():
					}
				case <-jw.Done():
				}
			}()

			srv := httptest.NewUnstartedServer(rq)
			srv.Config.ErrorLog = log.New(io.Discard, "", 0)
			srv.Start()
			defer srv.Close()
			u, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			initial.Host = u.Host
			initial.URL.Scheme = u.Scheme
			initial.URL.Host = u.Host

			hdr := http.Header{}
			hdr.Set("Origin", srv.URL)
			conn, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), &websocket.DialOptions{HTTPHeader: hdr})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.CloseNow() }()
			if resp.StatusCode != http.StatusSwitchingProtocols {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
			}

			select {
			case cleanup := <-cleanupCh:
				if cleanup.subscribed == nil {
					t.Fatal("subscribed channel is nil")
				}
				if cleanup.unsubscribed != cleanup.subscribed {
					t.Fatalf("unsubscribed channel %p, want %p", cleanup.unsubscribed, cleanup.subscribed)
				}
			case <-ctx.Done():
				t.Fatal("ConnectFn subscription was not released")
			}
		})
	}
}

func TestWS_ConnectFnFailureRefreshesClaimedSessionGrace(t *testing.T) {
	const nope = "nope"
	ts := newTestServer(t)
	defer ts.Close()
	ts.rq.SetConnectFn(func(_ *Request) error { return errors.New(nope) })

	ts.sess.mu.Lock()
	ts.sess.deadline = time.Now().Add(-time.Second)
	ts.sess.mu.Unlock()
	if ts.sess.isDead() {
		t.Fatal("attached session should not be dead before the claimed request ends")
	}

	conn, _, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if conn != nil {
		defer func() { _ = conn.CloseNow() }()
	}
	_, _, _ = conn.Read(ts.ctx)
	_ = conn.Close(websocket.StatusNormalClosure, "")
	waitForRequestCount(t, ts.jw, 0, testTimeout)

	if got := ts.jw.GetSession(ts.hr); got != ts.sess {
		t.Fatalf("claimed request should refresh session grace on early WebSocket failure, got %v", got)
	}
}

func TestWS_NormalExchange(t *testing.T) {
	th := newTestHelper(t)
	logger := &eventErrorLogger{}
	ts := newTestServerWithLogger(t, logger)
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
	loggedBeforeEvent := len(logger.loggedErrors())

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

	logged := logger.loggedErrors()
	if len(logged) != loggedBeforeEvent+1 {
		t.Fatalf("new logged errors = %v, want only the handler error", logged[loggedBeforeEvent:])
	}
	if !errors.Is(logged[loggedBeforeEvent], fooError) {
		t.Fatalf("logged error = %v, want %v", logged[loggedBeforeEvent], fooError)
	}
}

func TestWS_PingDisconnectsUnresponsiveClient(t *testing.T) {
	ts := newTestServer(t)
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
	ts := newTestServer(t)
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

// TestWS_SetContextCancellationClosesIdleConnection proves production
// reachability through a real WebSocket. The observed context confirms the
// request loop is already blocked on the old Done channel before SetContext
// installs and cancels a child context; the client must then observe the server
// close without sending any unrelated wake-up message.
func TestWS_SetContextCancellationClosesIdleConnection(t *testing.T) {
	ts := newTestServerNoSession(t)
	defer ts.Close()
	ts.jw.WebSocketPingInterval = 0

	observed := &observedDoneContext{
		Context:  ts.rq.Context(),
		observed: make(chan struct{}),
	}
	ts.rq.SetContext(func(context.Context) context.Context { return observed })

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}
	select {
	case <-ts.connectedCh:
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for websocket connect")
	}

	// Wake the loop once through a supported request-targeted broadcast. After it
	// sends the alert it re-enters select and observes the old context's Done.
	observed.armed.Store(true)
	ts.rq.Alert("info", "prime idle select")
	primeCtx, cancelPrime := context.WithTimeout(t.Context(), testTimeout)
	defer cancelPrime()
	if _, _, err = conn.Read(primeCtx); err != nil {
		t.Fatalf("reading priming alert: %v", err)
	}
	select {
	case <-observed.observed:
	case <-time.After(testTimeout):
		t.Fatal("request loop did not select on the old context")
	}

	ts.rq.SetContext(func(old context.Context) context.Context {
		ctx, cancel := context.WithCancel(old)
		cancel()
		return ctx
	})

	readCtx, cancelRead := context.WithTimeout(t.Context(), testTimeout)
	defer cancelRead()
	for readCtx.Err() == nil {
		if _, _, err = conn.Read(readCtx); err != nil {
			break
		}
	}
	if err == nil {
		t.Fatal("WebSocket remained open after replacement context cancellation")
	}
	if readCtx.Err() != nil {
		t.Fatalf("idle WebSocket closed only when the client read timed out: %v", err)
	}
	waitForRequestCount(t, ts.jw, 0, testTimeout)
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

// TestClearLockedZeroesWsQueue verifies clearLocked releases the queued wire
// message payloads before the Request is pooled, mirroring its clear() of todoDirt
// and elems. A bare [:0] reslice would leave the WsMsg values (including Data,
// which holds full Inner/Replace/Append HTML payloads) live in the backing array
// for the pooled Request's idle lifetime.
func TestClearLockedZeroesWsQueue(t *testing.T) {
	rq := &Request{tagMap: map[any][]*Element{}}
	rq.wsQueue = append(
		rq.wsQueue,
		wire.WsMsg{Data: "payload-a", Jid: 1, What: what.Inner},
		wire.WsMsg{Data: "payload-b", Jid: 2, What: what.Inner},
	)

	rq.clearLocked()

	if len(rq.wsQueue) != 0 {
		t.Fatalf("wsQueue len = %d, want 0", len(rq.wsQueue))
	}
	for i, m := range rq.wsQueue[:cap(rq.wsQueue)] {
		if m != (wire.WsMsg{}) {
			t.Errorf("wsQueue backing slot %d retained data after clearLocked: %+v", i, m)
		}
	}
}

// TestDelRequestNilsVacatedSlot verifies delRequest clears the freed tail slot so a
// session does not pin an otherwise-recyclable *Request in the slice backing array.
func TestDelRequestNilsVacatedSlot(t *testing.T) {
	t.Run("swap remove", func(t *testing.T) {
		sess := &Session{}
		rq1, rq2 := &Request{}, &Request{}
		sess.requests = []*Request{rq1, rq2}

		sess.delRequest(rq1)

		if len(sess.requests) != 1 || sess.requests[0] != rq2 {
			t.Fatalf("requests = %v, want [rq2]", sess.requests)
		}
		if got := sess.requests[:cap(sess.requests)][1]; got != nil {
			t.Errorf("vacated slot not nilled after swap-remove: %p", got)
		}
	})

	t.Run("last element", func(t *testing.T) {
		sess := &Session{}
		rq1 := &Request{}
		sess.requests = []*Request{rq1}

		sess.delRequest(rq1)

		if len(sess.requests) != 0 {
			t.Fatalf("requests len = %d, want 0", len(sess.requests))
		}
		if got := sess.requests[:cap(sess.requests)][0]; got != nil {
			t.Errorf("vacated slot not nilled after removing last element: %p", got)
		}
	})
}

// TestRequest_JawsKeyReadsAreLockedDuringRecycle verifies that the request-key
// readers used while the application renders the initial HTML page
// (JawsKeyString, String, HeadHTML and TailHTML) read rq.JawsKey under rq.mu, so
// they do not race the rq.mu-guarded writes to rq.JawsKey that clearLocked and
// getRequestLocked perform when a Request completes and its pooled storage is
// reused. Run with -race.
func TestRequest_JawsKeyReadsAreLockedDuringRecycle(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: mimic clearLocked / getRequestLocked, which assign rq.JawsKey while
	// holding rq.mu.
	go func() {
		defer wg.Done()
		for i := range iterations {
			rq.mu.Lock()
			rq.JawsKey = key.Key(uint64(i) + 1)
			rq.mu.Unlock()
		}
	}()

	// Reader: the lock-free render-path readers that previously read rq.JawsKey
	// without holding rq.mu.
	go func() {
		defer wg.Done()
		for range iterations {
			_ = rq.JawsKeyString()
			_ = rq.String()
			_ = rq.HeadHTML(io.Discard)
			_ = rq.TailHTML(io.Discard)
		}
	}()

	wg.Wait()
}

// TestServe_MarksRequestRunningSoMaintenanceSkips verifies that TestServe marks
// the request running before driving rq.process, mirroring ServeHTTP/startServe.
// The maintenance pass retires only not-running requests, so a request whose
// process loop is live must report running and survive a pass that would
// otherwise expire it.
func TestServe_MarksRequestRunningSoMaintenanceSkips(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	go jw.Serve()

	tr := NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}
	defer func() {
		tr.Close()
		<-tr.DoneCh
	}()

	<-tr.ReadyCh
	if !tr.running.Load() {
		t.Fatal("TestServe must mark the request running so maintenance cannot retire it mid-process")
	}

	// Make the request look long-idle, then run a maintenance pass directly. A
	// running request must remain registered while process is using it.
	tr.lastWriteSeconds.Store(jw.runtimeSeconds.Load() - 3600)
	jw.maintenance(time.Millisecond)

	if got := jw.RequestCount(); got != 1 {
		t.Fatalf("running request was retired by maintenance: RequestCount() = %d, want 1", got)
	}
}
