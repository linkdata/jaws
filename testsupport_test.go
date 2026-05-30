package jaws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
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

func TestNewTestRequest_PanicsWhenJawsClosed(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Close()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when the Jaws instance is closed")
		}
	}()
	NewTestRequest(jw, nil)
}

func TestTestServe_TimesOutWhenServeNotRunning(t *testing.T) {
	// Without a running Serve/ServeWithTimeout loop nothing drains subCh, so the
	// subscribe flush in TestServe can neither complete nor see Done, and it must
	// panic after its 5s timeout. Run in a synctest bubble so that timeout elapses
	// in fake time rather than stalling the test for five real seconds.
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		defer jw.Close()
		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		defer func() {
			s, ok := recover().(string)
			if !ok || !strings.Contains(s, "timed out subscribing") {
				t.Fatalf("expected timeout panic, got %v", s)
			}
		}()
		jw.TestServe(rq, func(any) {})
		t.Fatal("expected TestServe to panic")
	})
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
