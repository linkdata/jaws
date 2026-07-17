package jaws

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type connectUpdateTag string

type connectUpdateTestUI struct {
	entered chan<- struct{}
	release <-chan struct{}
	state   chan<- any
	value   any
	err     error
}

func (ui *connectUpdateTestUI) JawsRender(elem *Element, w io.Writer, _ []any) (err error) {
	elem.Tag(connectUpdateTag("ordered"))
	elem.SetConnectState("rendered")
	elem.JsCall("initial", "null")
	_, err = io.WriteString(w, `<div id="`+elem.Jid().String()+`"></div>`)
	return
}

func (*connectUpdateTestUI) JawsUpdate(*Element) {}

func (ui *connectUpdateTestUI) JawsConnectUpdate(elem *Element, renderedState any) (err error) {
	if ui.state != nil {
		ui.state <- renderedState
	}
	if ui.entered != nil {
		ui.entered <- struct{}{}
	}
	if ui.release != nil {
		<-ui.release
	}
	if err = ui.err; err == nil && ui.value != nil {
		err = elem.SetJsVar(ui.value)
	}
	return
}

func receiveConnectUpdateMessage(t *testing.T, ch <-chan wire.WsMsg) (msg wire.WsMsg) {
	t.Helper()
	select {
	case msg = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection update message")
	}
	return
}

func TestRequestProcess_OrdersConnectUpdateBeforeSubscribedBroadcast(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	state := make(chan any, 1)
	var releaseOnce sync.Once
	ui := &connectUpdateTestUI{
		entered: entered,
		release: release,
		state:   state,
		value:   "snapshot",
	}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(ui)
	if err = elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}

	inCh, outCh, bcastCh, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		close(inCh)
		<-doneCh
	}()
	defer releaseOnce.Do(func() { close(release) })
	<-readyCh
	<-entered
	if got := <-state; got != "rendered" {
		t.Fatalf("rendered connect state = %#v, want %q", got, "rendered")
	}

	// The broadcast subscription is already installed, but process is paused in
	// the state snapshot. Buffer a normal subscribed update on the exact channel
	// that production Serve registered, then let the snapshot finish.
	bcastCh <- wire.Message{
		Dest: connectUpdateTag("ordered"),
		What: what.Set,
		Data: `="broadcast"`,
	}
	releaseOnce.Do(func() { close(release) })

	want := []wire.WsMsg{
		{Jid: elem.Jid(), What: what.Call, Data: "initial=null"},
		{Jid: elem.Jid(), What: what.Set, Data: `="snapshot"`},
		{Jid: elem.Jid(), What: what.Set, Data: `="broadcast"`},
	}
	for i := range want {
		if got := receiveConnectUpdateMessage(t, outCh); got != want[i] {
			t.Fatalf("message %d = %#v, want %#v", i, got, want[i])
		}
	}
}

func TestRequestProcess_ConnectUpdateErrorCancelsBeforeSend(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	wantErr := errors.New("connect update failed")
	ui := &connectUpdateTestUI{err: wantErr}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	ctx := rq.Context()
	if err = rq.NewElement(ui).JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}

	inCh, outCh, _, _, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer close(inCh)
	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request cancellation")
	}
	if !errors.Is(context.Cause(ctx), wantErr) {
		t.Fatalf("request cancellation cause = %v, want %v", context.Cause(ctx), wantErr)
	}
	select {
	case msg, ok := <-outCh:
		if ok {
			t.Fatalf("connect update error sent queued message %#v", msg)
		}
	default:
		t.Fatal("outbound channel remained open after process stopped")
	}
}

func TestElementSetJsVar(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	ui := &connectUpdateTestUI{}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(ui)
	if err = elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if err = elem.SetJsVar(nil); err != nil {
		t.Fatal(err)
	}
	if got := rq.wsQueue[len(rq.wsQueue)-1]; got.What != what.Set || got.Jid != elem.Jid() || got.Data != "=null" {
		t.Fatalf("SetJsVar(nil) queued %#v", got)
	}
	before := len(rq.wsQueue)
	if err = elem.SetJsVar(make(chan int)); err == nil {
		t.Fatal("expected JSON marshal error")
	}
	if got := len(rq.wsQueue); got != before {
		t.Fatalf("SetJsVar queue length = %d, want %d", got, before)
	}
}

func TestElementSetConnectStateAfterRenderPanics(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(&connectUpdateTestUI{})
	if err = elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected late SetConnectState to report API misuse")
		}
	}()
	elem.SetConnectState("late")
}

func TestElementSetConnectStateDeletedElementIsInert(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(&connectUpdateTestUI{})
	rq.DeleteElement(elem)
	elem.SetConnectState("ignored")
	if state := elem.takeConnectState(); state != nil {
		t.Fatalf("deleted Element retained connect state %#v", state)
	}
}

func TestRequestProcess_NilSubscriptionSkipsConnectUpdates(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	entered := make(chan struct{}, 1)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if err = rq.NewElement(&connectUpdateTestUI{entered: entered}).JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	inCh := make(chan wire.WsMsg)
	close(inCh)
	outCh := make(chan wire.WsMsg, 1)
	rq.process(nil, inCh, outCh)
	select {
	case <-entered:
		t.Fatal("nil broadcast subscription invoked ConnectUpdater")
	default:
	}
	if _, ok := <-outCh; ok {
		t.Fatal("outbound channel remained open after process stopped")
	}
}

func TestRequestQueueConnectUpdates_DropsDynamicElementState(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if err = rq.queueConnectUpdates(); err != nil {
		t.Fatal(err)
	}
	if err = rq.queueConnectUpdates(); err != nil {
		t.Fatalf("second connection update pass = %v", err)
	}
	elem := rq.NewElement(&connectUpdateTestUI{})
	if err = elem.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if state := elem.takeConnectState(); state != nil {
		t.Fatalf("dynamic Element retained connect state %#v after reconciliation", state)
	}
}

func TestTestServe_ClosesConnectSnapshotBeforeReturn(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	// Drive TestServe's subscription rendezvous directly, and hold jw.mu so its
	// process goroutine cannot publish Ready before TestServe returns. This makes
	// the synchronous snapshot boundary observable without a scheduler race.
	unsubscribed := make(chan struct{})
	go func() {
		<-jw.subCh
		<-jw.subCh
		<-jw.unsubCh
		close(unsubscribed)
	}()
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	jw.mu.Lock()
	inCh, _, _, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	started := rq.connectStarted.Load()
	elem := rq.NewElement(&connectUpdateTestUI{})
	renderErr := elem.JawsRender(io.Discard, nil)
	state := elem.takeConnectState()
	jw.mu.Unlock()

	<-readyCh
	close(inCh)
	<-doneCh
	<-unsubscribed
	if renderErr != nil {
		t.Fatal(renderErr)
	}
	if !started {
		t.Fatal("TestServe returned while connection snapshots were still accepted")
	}
	if state != nil {
		t.Fatalf("Element rendered after TestServe retained connection state %#v", state)
	}
}

func TestRequestQueueConnectUpdates_SkipsFrozenUnrenderedElement(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	entered := make(chan struct{}, 1)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(&connectUpdateTestUI{entered: entered})
	elem.SetConnectState("update-only")
	elem.Freeze()
	if err = rq.queueConnectUpdates(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
		t.Fatal("ConnectUpdater ran for an Element that was frozen without rendering")
	default:
	}
	if state := elem.takeConnectState(); state != nil {
		t.Fatalf("frozen unrendered Element retained connect state %#v", state)
	}
}
