package jaws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestErrEventUnhandled_Error(t *testing.T) {
	if got := ErrEventUnhandled.Error(); got != "event unhandled" {
		t.Fatalf("ErrEventUnhandled.Error() = %q, want %q", got, "event unhandled")
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

func TestNewTestRequest_SuccessPathAndClose(t *testing.T) {
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

	if tr.Initial() == nil {
		t.Fatal("expected initial request")
	}

	tr.Close()
	select {
	case <-tr.DoneCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for test request shutdown")
	}
}
