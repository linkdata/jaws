package jaws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// TestEarlyCallbackPreservesInitialRenderIdentity is the reproduction from issue
// #195: an early /jaws/<key> callback that fails the WebSocket upgrade tears the
// Request down — clearing its collections — but must not destroy the identity the
// initial HTTP handler is still rendering with. Because Requests keep a stable
// identity and are never reused, completion unregisters the Request and releases its
// buffers but never zeroes the key or hands the pointer to another connection, so the
// initial renderer's pointer keeps a valid key.
func TestEarlyCallbackPreservesInitialRenderIdentity(t *testing.T) {
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
		t.Fatal("early callback cleared the initial render's Request key")
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
// where the initial renderer keeps registering after the Request finished. This must
// not panic and must not leak into a later, distinct Request. Registration is not a
// full no-op: NewElement still creates an Element and advances the Jid, while Tag
// reaches the nil-map guard in TagExpanded (the buffers were released) and is dropped
// — neither panics, and none of it is visible to another Request.
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

// TestRequestFinishConcurrentWithRenderIsRaceFree drives a live initial render
// (NewElement, Tag and JawsRender, which reads Element.ui and handlers lock-free)
// concurrently with completion across many Requests. Completion must not mutate the
// render-visible Element fields or the Jid counter, so this is race-free under -race,
// and the nil-map guard keeps a post-completion Tag from panicking.
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
				elem := rq.NewElement(&testUi{})
				rq.Tag(elem, tag.Tag("t"))
				_ = elem.JawsRender(io.Discard, nil)
			}
		}()
		go func() {
			defer wg.Done()
			jw.recycle(rq)
		}()
	}
	wg.Wait()
}

// TestFinishDoesNotResetJidCounter verifies teardown leaves the Jid counter intact,
// so a renderer that keeps allocating Elements after a racy teardown cannot reuse an
// already-streamed Jid. Restoring lastJid = 0 in teardown would fail this.
func TestFinishDoesNotResetJidCounter(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rq.NewElement(&testUi{})
	last := rq.NewElement(&testUi{}).Jid()

	jw.recycle(rq)

	if got := rq.NewElement(&testUi{}).Jid(); got <= last {
		t.Fatalf("Jid after teardown = %v, want > %v (counter must not reset)", got, last)
	}
}

// TestRecycleQueueRaceDoesNotLeak stresses a queue concurrent with recycle. The
// wsQueue transfer in releaseBuffersLocked must stay under muQueue, so a late queue
// cannot race the transfer or land its message in a buffer already returned to the
// pool. Moving the transfer outside muQueue would trip the race detector here. Run
// with -race.
func TestRecycleQueueRaceDoesNotLeak(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	const n = 300
	for range n {
		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			rq.queue(wire.WsMsg{Jid: 1, What: what.Inner, Data: "late"})
		}()
		go func() {
			defer wg.Done()
			jw.recycle(rq)
		}()
		wg.Wait()
	}
}

// TestNoscriptDuringLiveRenderRecordsJavascriptDisabled guards against re-adding a
// render-timing gate that would 404 the <noscript> probe while a streamed page is
// still rendering (its initial request context still live). The probe must reach the
// Request, return 204, and record ErrJavascriptDisabled.
func TestNoscriptDuringLiveRenderRecordsJavascriptDisabled(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()
	waitForServeLoop(t, jw)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rq := jw.NewRequest(initial)

	w := httptest.NewRecorder()
	probe := httptest.NewRequest(http.MethodGet, "/jaws/"+rq.JawsKeyString()+"/noscript", nil)
	probe.RemoteAddr = initial.RemoteAddr
	jw.ServeHTTP(w, probe)

	if w.Code != http.StatusNoContent {
		t.Fatalf("/noscript during live render: status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if cause := context.Cause(rq.Context()); !errors.Is(cause, ErrJavascriptDisabled) {
		t.Fatalf("cancellation cause = %v, want ErrJavascriptDisabled", cause)
	}
}

// TestRequestRecycledKeyNotReusedWhileReachable proves the recycle path reserves the
// key with a tombstone: with the key generator forced to offer K, K, then K2, a
// Request minted K and then recycled must not have K reassigned to the next Request
// while the finished one is still reachable; the next Request gets K2 instead.
func TestRequestRecycledKeyNotReusedWhileReachable(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	const k = key.Key(0x1111111111111111)
	const k2 = key.Key(0x2222222222222222)
	var kb, k2b [8]byte
	binary.LittleEndian.PutUint64(kb[:], uint64(k))
	binary.LittleEndian.PutUint64(k2b[:], uint64(k2))
	stream := append(append(append([]byte{}, kb[:]...), kb[:]...), bytes.Repeat(k2b[:], 4)...)
	jw.mu.Lock()
	jw.kg = bufio.NewReader(bytes.NewReader(stream))
	jw.mu.Unlock()

	rq1 := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq1.JawsKey != k {
		t.Fatalf("rq1 key = %v, want forced %v", rq1.JawsKey, k)
	}
	jw.recycle(rq1)

	rq2 := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	defer jw.recycle(rq2)
	if rq2.JawsKey == k {
		t.Fatal("recycled key K was reassigned while the finished Request is still reachable")
	}
	if rq2.JawsKey != k2 {
		t.Fatalf("rq2 key = %v, want %v (K skipped via tombstone)", rq2.JawsKey, k2)
	}
	runtime.KeepAlive(rq1) // keep rq1 reachable so its tombstone is not GC-cleaned mid-test
}
