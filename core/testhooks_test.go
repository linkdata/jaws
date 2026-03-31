package jaws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkdata/jaws/core/wire"
	"github.com/linkdata/jaws/what"
)

func TestErrEventUnhandled_Error(t *testing.T) {
	if got := ErrEventUnhandled.Error(); got != "event unhandled" {
		t.Fatalf("ErrEventUnhandled.Error() = %q, want %q", got, "event unhandled")
	}
}

func TestTestHooks_RequestLifecycle(t *testing.T) {
	tj := newTestJaws()
	defer tj.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := tj.NewRequest(hr)
	if got := tj.UseRequest(rq.JawsKey, hr); got != rq {
		t.Fatalf("UseRequest returned %p, want %p", got, rq)
	}

	msgCh := tj.SubscribeForTest(rq, 2)
	if msgCh == nil {
		t.Fatal("SubscribeForTest returned nil channel")
	}

	pumped := make(chan struct{})
	go func() {
		tj.PumpSubscriptionsForTest()
		close(pumped)
	}()
	select {
	case <-pumped:
	case <-time.After(time.Second):
		t.Fatal("PumpSubscriptionsForTest timed out")
	}

	tj.Broadcast(wire.Message{What: what.Reload, Data: "reloaded"})
	select {
	case got := <-msgCh:
		if got.What != what.Reload || got.Data != "reloaded" {
			t.Fatalf("broadcast got %v, want Reload/reloaded", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}

	incoming := make(chan wire.WsMsg)
	outbound := make(chan wire.WsMsg, 1)
	processed := make(chan struct{})
	go func() {
		rq.ProcessForTest(msgCh, incoming, outbound)
		close(processed)
	}()
	close(incoming)

	select {
	case <-processed:
	case <-time.After(time.Second):
		t.Fatal("ProcessForTest did not return")
	}

	select {
	case _, ok := <-outbound:
		if ok {
			t.Fatal("expected outbound channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for outbound close")
	}

	tj.RecycleForTest(rq)
	if got := tj.RequestCount(); got != 0 {
		t.Fatalf("RequestCount() = %d, want 0", got)
	}
}

func TestNewRequestHarness_ReturnsNilOnClaimFailure(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	jw.reqPool.New = func() any {
		rq := (&Request{
			Jaws:   jw,
			tagMap: make(map[any][]*Element),
		}).clearLocked()
		rq.claimed.Store(true)
		return rq
	}

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	if rh := newRequestHarness(jw, hr); rh != nil {
		t.Fatal("expected nil harness when claim fails")
	}
}
