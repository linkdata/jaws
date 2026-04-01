package jaws

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
