package jaws

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/tag"
)

// TestEarlyCallbackDoesNotRecycleInitialRender is the reproduction from issue #195:
// an early /jaws/<key> callback that fails the WebSocket upgrade must not clear the
// Request the initial HTTP handler is still rendering with. Because Requests keep a
// stable identity and are never reused, the callback's stopServe completion only
// unregisters and releases buffers; it never zeroes the key or hands the pointer to
// another connection, so the initial renderer keeps a valid key.
func TestEarlyCallbackDoesNotRecycleInitialRender(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	initial := httptest.NewRequest(http.MethodGet, "/", nil)
	initial.RemoteAddr = "192.0.2.1:1000"
	rq := jw.NewRequest(initial)

	var page bytes.Buffer
	if err := rq.HeadHTML(&page); err != nil {
		t.Fatal(err)
	}

	callback := httptest.NewRequest(http.MethodGet, "/jaws/"+rq.JawsKeyString(), nil)
	callback.RemoteAddr = initial.RemoteAddr
	jw.ServeHTTP(httptest.NewRecorder(), callback)

	if rq.JawsKeyString() == "" {
		t.Fatal("Request was cleared while the initial render still owned it")
	}
}

// TestRequestLateCancelDoesNotReachNextConnection proves the core identity-stability
// guarantee: after a Request's WebSocket connects and disconnects, a later
// NewRequest returns a distinct Request, the finished Request keeps its key, and a
// late Cancel on the finished Request cannot cancel the later one.
func TestRequestLateCancelDoesNotReachNextConnection(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()
	waitForServeLoop(t, jw)

	server := httptest.NewServer(jw)
	defer server.Close()
	newInitialRequest := func(path string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, server.URL+path, nil)
		r.RemoteAddr = "127.0.0.1:12345"
		return r
	}

	finished := jw.NewRequest(newInitialRequest("/first"))
	finishedKey := finished.JawsKey
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/jaws/"+finished.JawsKeyString(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{server.URL}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
	waitForRequestCount(t, jw, 0, time.Second)

	replacement := jw.NewRequest(newInitialRequest("/second"))
	defer jw.recycle(replacement)
	if replacement == finished {
		t.Fatal("NewRequest reused the finished Request identity")
	}
	if finished.JawsKey != finishedKey {
		t.Fatalf("finished Request key = %v, want stable key %v", finished.JawsKey, finishedKey)
	}
	replacementCtx := replacement.Context()
	finished.Cancel(errors.New("background operation completed after disconnect"))
	select {
	case <-replacementCtx.Done():
		t.Fatalf("late Cancel reached the next Request: %v", context.Cause(replacementCtx))
	default:
	}
}

// TestRequestFinishDoesNotPanicOnContinuedRender exercises the racy #195 window
// where the initial renderer keeps registering after the Request finished. Element
// and tag registration on a finished Request (whose buffers were released, leaving a
// nil tagMap) must be an inert no-op rather than a nil-map panic, and must not leak
// into a later, distinct Request.
func TestRequestFinishDoesNotPanicOnContinuedRender(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rq.Tag(rq.NewElement(&testUi{}), tag.Tag("live"))

	// Finish the Request out from under the still-running initial renderer, as a racy
	// early /jaws/<key> callback would.
	jw.recycle(rq)

	// The renderer keeps going. A fresh element created after completion is not marked
	// deleted, so tagging it reaches the nil-map guard in TagExpanded; this must not
	// panic.
	late := rq.NewElement(&testUi{})
	rq.Tag(late, tag.Tag("late"))
	rq.Dirty(tag.Tag("live"))

	other := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/next", nil))
	defer jw.recycle(other)
	if other == rq {
		t.Fatal("NewRequest reused a finished Request identity")
	}
	if got := len(other.GetElements(tag.Tag("live"))) + len(other.GetElements(tag.Tag("late"))); got != 0 {
		t.Fatalf("finished Request's elements leaked into a later Request: %d", got)
	}
}

// TestRequestFinishConcurrentWithRenderIsRaceFree drives element and tag
// registration concurrently with completion across many Requests. Both paths take
// rq.mu, so this must be race-free under -race, and the nil-map guard must keep a
// post-completion registration from panicking.
func TestRequestFinishConcurrentWithRenderIsRaceFree(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	const n = 200
	var wg sync.WaitGroup
	for range n {
		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 8 {
				rq.Tag(rq.NewElement(&testUi{}), tag.Tag("t"))
			}
		}()
		go func() {
			defer wg.Done()
			jw.recycle(rq)
		}()
	}
	wg.Wait()
}
