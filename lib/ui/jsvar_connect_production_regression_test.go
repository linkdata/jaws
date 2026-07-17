package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type connectMarshalValue struct {
	Panic bool
	Value any
}

type connectWrappedJsVar struct {
	*JsVar[jsVarData]
}

type connectValueWrappedJsVar struct {
	*JsVar[jsVarData]
	marker int
}

type connectBarrierRWMutex struct {
	sync.RWMutex
	armed    atomic.Bool
	unlocked chan struct{}
	release  chan struct{}
}

func (mu *connectBarrierRWMutex) RUnlock() {
	mu.RWMutex.RUnlock()
	if mu.armed.CompareAndSwap(true, false) {
		close(mu.unlocked)
		<-mu.release
	}
}

func (value *connectMarshalValue) MarshalJSON() ([]byte, error) {
	if value.Panic {
		panic("connect marshal panic")
	}
	return json.Marshal(value.Value)
}

func TestJsVar_ConnectUpdateClosesRenderToSubscribeGap(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	active := jawstest.NewTestRequest(jw, nil)
	defer func() {
		active.Close()
		<-active.DoneCh
	}()
	<-active.ReadyCh

	var mu sync.RWMutex
	value := jsVarData{Text: "rendered", Num: 1}
	jsvar := NewJsVar(&mu, &value)
	activeElem := active.NewElement(jsvar)
	if err = activeElem.JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
		t.Fatal(err)
	}

	pending := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	pendingElem := pending.NewElement(jsvar)
	var rendered strings.Builder
	if err = pendingElem.JawsRender(&rendered, []any{"shared"}); err != nil {
		t.Fatal(err)
	}
	if got := htmlAttrValue(t, rendered.String(), "data-jawsdata"); got != `{"text":"rendered","num":1}` {
		t.Fatalf("pending initial data = %s", got)
	}

	// This is an ordinary server-side write through the documented JsVar API.
	// Broadcasts intentionally target active requests, so the pending request
	// cannot receive this frame through the normal distribution path.
	if err = jsvar.JawsSetPath(activeElem, "text", "current"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-active.OutCh:
		if msg.What != what.Set || msg.Jid != activeElem.Jid() || msg.Data != `text="current"` {
			t.Fatalf("active broadcast = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for active JsVar broadcast")
	}

	inCh, outCh, _, readyCh, doneCh := jw.TestServe(pending, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		close(inCh)
		<-doneCh
	}()
	<-readyCh

	select {
	case msg := <-outCh:
		want := wire.WsMsg{
			Jid:  pendingElem.Jid(),
			What: what.Set,
			Data: `={"text":"current","num":1}`,
		}
		if msg != want {
			t.Fatalf("pending connection update = %#v, want %#v", msg, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending JsVar connection update")
	}

	// A request-key barrier proves the one-request catch-up did not leak back to
	// the already-current peer.
	jw.JsCall(active.JawsKey, "jsVarConnectBarrier", "null")
	select {
	case msg := <-active.OutCh:
		if msg.What != what.Call || msg.Data != "jsVarConnectBarrier=null" {
			t.Fatalf("connection update leaked to active peer before barrier: %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for active-request barrier")
	}
}

func TestJsVar_ConnectUpdateOrdersMutationAfterSnapshot(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	active := jawstest.NewTestRequest(jw, nil)
	defer func() {
		active.Close()
		<-active.DoneCh
	}()
	<-active.ReadyCh

	mu := &connectBarrierRWMutex{
		unlocked: make(chan struct{}),
		release:  make(chan struct{}),
	}
	var releaseOnce sync.Once
	value := jsVarData{Text: "rendered", Num: 1}
	jsvar := NewJsVar(mu, &value)
	activeElem := active.NewElement(jsvar)
	if err = activeElem.JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
		t.Fatal(err)
	}

	pending := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	pendingElem := pending.NewElement(jsvar)
	if err = pendingElem.JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
		t.Fatal(err)
	}
	pendingElem.JsCall("initial", "null")

	if err = jsvar.JawsSetPath(activeElem, "text", "snapshot"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-active.OutCh:
		if msg.What != what.Set || msg.Data != `text="snapshot"` {
			t.Fatalf("snapshot broadcast = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for snapshot broadcast")
	}

	mu.armed.Store(true)
	inCh, outCh, _, readyCh, doneCh := jw.TestServe(pending, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		releaseOnce.Do(func() { close(mu.release) })
		close(inCh)
		<-doneCh
	}()
	<-readyCh
	select {
	case <-mu.unlocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection snapshot unlock")
	}

	// The hook has captured "snapshot" and released the value lock, but it is
	// paused before queueing that root update. Apply a normal later mutation and
	// prove Serve has distributed it to every subscription before releasing the
	// hook.
	if err = jsvar.JawsSetPath(activeElem, "text", "latest"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-active.OutCh:
		if msg.What != what.Set || msg.Data != `text="latest"` {
			t.Fatalf("latest broadcast = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for latest broadcast")
	}
	jw.JsCall(active.JawsKey, "snapshotBarrier", "null")
	select {
	case msg := <-active.OutCh:
		if msg.What != what.Call || msg.Data != "snapshotBarrier=null" {
			t.Fatalf("snapshot barrier = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for snapshot barrier")
	}
	releaseOnce.Do(func() { close(mu.release) })

	want := []wire.WsMsg{
		{Jid: pendingElem.Jid(), What: what.Call, Data: "initial=null"},
		{Jid: pendingElem.Jid(), What: what.Set, Data: `={"text":"snapshot","num":1}`},
		{Jid: pendingElem.Jid(), What: what.Set, Data: `text="latest"`},
	}
	for i := range want {
		select {
		case msg := <-outCh:
			if msg != want[i] {
				t.Fatalf("pending message %d = %#v, want %#v", i, msg, want[i])
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for pending message %d", i)
		}
	}
}

func TestJsVar_ConnectUpdateSkipsUnchangedValue(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	var mu sync.RWMutex
	value := jsVarData{Text: "unchanged", Num: 1}
	jsvar := NewJsVar(&mu, &value)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if err = rq.NewElement(jsvar).JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
		t.Fatal(err)
	}

	inCh, outCh, _, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		close(inCh)
		<-doneCh
	}()
	<-readyCh

	jw.JsCall(rq.JawsKey, "unchangedBarrier", "null")
	select {
	case msg := <-outCh:
		if msg.What != what.Call || msg.Data != "unchangedBarrier=null" {
			t.Fatalf("unchanged JsVar queued a connection update before barrier: %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for unchanged-value barrier")
	}
}

func TestJsVar_ConnectUpdateReconcilesPtrBecomingNil(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	var mu sync.RWMutex
	value := jsVarData{Text: "rendered", Num: 1}
	jsvar := NewJsVar(&mu, &value)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(jsvar)
	if err = elem.JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
		t.Fatal(err)
	}
	jsvar.Lock()
	jsvar.Ptr = nil
	jsvar.Unlock()

	inCh, outCh, _, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer func() {
		close(inCh)
		<-doneCh
	}()
	<-readyCh

	select {
	case msg := <-outCh:
		want := wire.WsMsg{Jid: elem.Jid(), What: what.Set, Data: "=null"}
		if msg != want {
			t.Fatalf("nil Ptr connection update = %#v, want %#v", msg, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for nil Ptr connection update")
	}
}

func TestJsVar_ConnectUpdateSkipsMissingSnapshotAndOriginallyNilPtr(t *testing.T) {
	_, rq := newCoreRequest(t)

	var mu sync.RWMutex
	jsvar := NewJsVar[jsVarData](&mu, nil)
	elem := rq.NewElement(jsvar)
	if err := jsvar.JawsConnectUpdate(elem, nil); err != nil {
		t.Fatal(err)
	}
	if err := jsvar.JawsConnectUpdate(elem, nilJsVarConnectState{}); err != nil {
		t.Fatal(err)
	}
}

func TestJsVar_ConnectUpdateSkipsPromotedCompositeMethod(t *testing.T) {
	for _, tc := range []struct {
		name string
		wrap func(*JsVar[jsVarData]) jaws.UI
	}{
		{
			name: "pointer",
			wrap: func(jsvar *JsVar[jsVarData]) jaws.UI {
				return &connectWrappedJsVar{JsVar: jsvar}
			},
		},
		{
			name: "value",
			wrap: func(jsvar *JsVar[jsVarData]) jaws.UI {
				return connectValueWrappedJsVar{JsVar: jsvar, marker: 1}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)
			go jw.Serve()

			var mu sync.RWMutex
			value := jsVarData{Text: "rendered", Num: 1}
			jsvar := NewJsVar(&mu, &value)
			rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
			elem := rq.NewElement(tc.wrap(jsvar))
			if err = elem.JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			value.Text = "changed"
			mu.Unlock()

			inCh, outCh, _, readyCh, doneCh := jw.TestServe(rq, func(recovered any) {
				if recovered != nil {
					panic(recovered)
				}
			})
			defer func() {
				close(inCh)
				<-doneCh
			}()
			<-readyCh

			jw.JsCall(rq.JawsKey, "compositeBarrier", "null")
			select {
			case msg := <-outCh:
				if msg.What != what.Call || msg.Data != "compositeBarrier=null" {
					t.Fatalf("promoted JsVar method queued a partial root update: %#v", msg)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for composite barrier")
			}
		})
	}
}

func TestJsVar_ConnectUpdateSizeCapCancelsRequest(t *testing.T) {
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	go jw.Serve()

	old := MaxClientJsVarBytes
	MaxClientJsVarBytes = 64
	defer func() { MaxClientJsVarBytes = old }()

	var mu sync.RWMutex
	value := jsVarData{Text: "small", Num: 1}
	jsvar := NewJsVar(&mu, &value)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	ctx := rq.Context()
	elem := rq.NewElement(jsvar)
	if err = elem.JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
		t.Fatal(err)
	}
	if err = jsvar.JawsSetPath(elem, "text", strings.Repeat("z", 256)); err != nil {
		t.Fatal(err)
	}

	inCh, _, _, _, doneCh := jw.TestServe(rq, func(recovered any) {
		if recovered != nil {
			panic(recovered)
		}
	})
	defer close(inCh)
	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for oversized JsVar cancellation")
	}
	if cause := context.Cause(ctx); !errors.Is(cause, ErrJsVarTooLarge) {
		t.Fatalf("oversized JsVar cancellation cause = %v, want ErrJsVarTooLarge", cause)
	}
}

func TestJsVar_ConnectUpdateMarshalPanicReleasesValueLock(t *testing.T) {
	jw, rq := newCoreRequest(t)
	go jw.Serve()

	var mu sync.RWMutex
	value := connectMarshalValue{Value: "rendered"}
	jsvar := NewJsVar(&mu, &value)
	elem := rq.NewElement(jsvar)
	if err := elem.JawsRender(&strings.Builder{}, []any{"shared"}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	value.Panic = true
	mu.Unlock()
	func() {
		defer func() {
			if recovered := recover(); recovered != "connect marshal panic" {
				t.Fatalf("marshal panic = %v", recovered)
			}
		}()
		_ = jsvar.JawsConnectUpdate(elem, struct{}{})
	}()
	if !mu.TryLock() {
		t.Fatal("JsVar read lock remained held after connection marshal panic")
	}
	mu.Unlock()
}
