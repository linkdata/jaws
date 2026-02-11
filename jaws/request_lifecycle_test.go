package jaws

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
)

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
	jw.unsubCh <- make(chan Message)
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
	bcastCh := make(chan Message)
	inCh := make(chan WsMsg)
	outCh := make(chan WsMsg, 1)
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
	jw.Broadcast(Message{What: what.Update})
}
