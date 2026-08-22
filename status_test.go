package jaws

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func newStatusTestJaws(t *testing.T, configure func(*Jaws)) *Jaws {
	t.Helper()
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// These tests drive maintenance explicitly and inspect its dirt, so prevent
	// the independent browser-update ticker from draining that dirt.
	jw.updateTicker.Stop()
	configure(jw)
	serveDone := make(chan struct{})
	go func() {
		jw.Serve()
		close(serveDone)
	}()
	t.Cleanup(func() {
		jw.Close()
		select {
		case <-serveDone:
		case <-time.After(testTimeout):
			t.Error("timeout waiting for Jaws Serve loop")
		}
	})
	waitForServeLoop(t, jw)
	return jw
}

func requireDirtyTags(t *testing.T, jw *Jaws, want ...any) {
	t.Helper()
	got := make(map[any]struct{})
	jw.mu.Lock()
	for tagValue := range jw.dirty {
		got[tagValue] = struct{}{}
	}
	clear(jw.dirty)
	jw.dirtOrder = 0
	jw.mu.Unlock()
	if len(got) != len(want) {
		t.Fatalf("dirty tags = %v, want %v", got, want)
	}
	for _, tagValue := range want {
		if _, ok := got[tagValue]; !ok {
			t.Fatalf("dirty tags = %v, want %v", got, want)
		}
	}
}

func requireUpdateList(t *testing.T, rq *Request, want ...*Element) {
	t.Helper()
	if got := rq.makeUpdateList(); !slices.Equal(got, want) {
		t.Fatalf("update list = %v, want %v", got, want)
	}
}

func dialJawsRequest(t *testing.T, serverURL string, rq *Request) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("Origin", serverURL)
	requestURL := serverURL + "/jaws/" + rq.JawsKeyString()
	conn, response, err := websocket.Dial(t.Context(), requestURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("dialing %s: status=%d err=%v", requestURL, status, err)
	}
	return conn
}

func TestJaws_StatusMetrics(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	if got := jw.StatusMetrics.Load(); got != 0 {
		t.Fatalf("default StatusMetrics = %v, want 0", got)
	}

	initial := httptest.NewRequest(http.MethodGet, "/", nil)
	first := jw.newRequest(initial)
	if sess := jw.NewSession(httptest.NewRecorder(), initial); sess == nil {
		t.Fatal("NewSession returned nil")
	}
	_ = jw.Log(errors.New("disabled"))
	if got := jw.ErrorCount(); got != 1 {
		t.Fatalf("ErrorCount() with status updates disabled = %d, want 1", got)
	}
	jw.StatusMetrics.Store(1 << 31)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw)

	selected := StatusMetricPendingRequests | StatusMetricErrors
	jw.StatusMetrics.Store(selected)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.PendingRequestCountTag(), jw.ErrorCountTag())
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw)

	if got := jw.UseRequest(first.JawsKey, initial); got != first {
		t.Fatalf("UseRequest() = %p, want %p", got, first)
	}
	_ = jw.Log(errors.New("enabled"))
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.PendingRequestCountTag(), jw.ErrorCountTag())

	jw.StatusMetrics.And(0)
	jw.newRequest(httptest.NewRequest(http.MethodGet, "/second", nil))
	_ = jw.Log(errors.New("disabled again"))
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw)

	jw.StatusMetrics.Or(selected)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.PendingRequestCountTag(), jw.ErrorCountTag())

	jw.StatusMetrics.Store(StatusMetricAll)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw,
		jw.ActiveRequestCountTag(),
		jw.SessionCountTag(),
		jw.ActiveSessionCountTag(),
	)
}

func TestJaws_StatusTags(t *testing.T) {
	first, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Close)
	second, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Close)

	getters := []func(*Jaws) any{
		(*Jaws).ActiveRequestCountTag,
		(*Jaws).PendingRequestCountTag,
		(*Jaws).SessionCountTag,
		(*Jaws).ActiveSessionCountTag,
		(*Jaws).ErrorCountTag,
	}
	seen := make(map[any]struct{}, len(getters)*2)
	for _, getter := range getters {
		firstTag := getter(first)
		if got := getter(first); got != firstTag {
			t.Fatalf("tag changed from %p to %p", firstTag, got)
		}
		secondTag := getter(second)
		for _, tagValue := range []any{firstTag, secondTag} {
			if _, ok := seen[tagValue]; ok {
				t.Fatalf("duplicate status tag %p", tagValue)
			}
			seen[tagValue] = struct{}{}
		}
	}

	rq := second.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	foreign := rq.NewElement(new(testUi))
	own := rq.NewElement(new(testUi))
	foreign.Tag(first.ActiveRequestCountTag())
	own.Tag(second.ActiveRequestCountTag())
	second.StatusMetrics.Store(StatusMetricActiveRequests)
	second.maintenance(time.Hour)
	if got := second.distributeDirt(); got != 1 {
		t.Fatalf("distributed selectors = %d, want 1", got)
	}
	requireUpdateList(t, rq, own)
}

func TestJaws_ActiveRequestCountTag(t *testing.T) {
	ts := newTestServerNoSession(t)
	defer ts.Close()
	jw := ts.jw
	jw.StatusMetrics.Store(StatusMetricActiveRequests)

	// Drain the first enabled sample before registering the observer Element.
	jw.maintenance(time.Hour)
	jw.distributeDirt()
	requireUpdateList(t, ts.rq)

	activeValues := make(chan int, 3)
	statusElem := ts.rq.NewElement(&testUi{updateFn: func(*Element) {
		_, active := jw.RequestCounts()
		activeValues <- active
	}})
	statusElem.Tag(jw.ActiveRequestCountTag())

	wantActive := func(want int) {
		t.Helper()
		select {
		case got := <-activeValues:
			if got != want {
				t.Fatalf("rendered active Request count = %d, want %d", got, want)
			}
		case <-time.After(testTimeout):
			t.Fatalf("timeout waiting for active Request count %d", want)
		}
	}

	observerConn, _, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := observerConn.CloseNow(); closeErr != nil {
			t.Errorf("closing observer WebSocket: %v", closeErr)
		}
	}()
	select {
	case <-ts.connectedCh:
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for observer WebSocket")
	}
	jw.maintenance(time.Hour)
	wantActive(1)

	targetInitial := httptest.NewRequest(http.MethodGet, ts.srv.URL+"/target", nil)
	targetInitial.RemoteAddr = "127.0.0.1:1"
	target := jw.NewRequest(httptest.NewRecorder(), targetInitial)
	if target == nil {
		t.Fatal("NewRequest returned nil")
	}
	targetConnected := make(chan struct{})
	target.SetConnectFn(func(*Request) error {
		close(targetConnected)
		return nil
	})
	targetConn := dialJawsRequest(t, ts.srv.URL, target)
	targetClosed := false
	defer func() {
		if !targetClosed {
			if closeErr := targetConn.CloseNow(); closeErr != nil {
				t.Errorf("closing target WebSocket: %v", closeErr)
			}
		}
	}()
	select {
	case <-targetConnected:
	case <-time.After(testTimeout):
		t.Fatal("timeout waiting for target WebSocket")
	}
	jw.maintenance(time.Hour)
	wantActive(2)

	if closeErr := targetConn.Close(websocket.StatusNormalClosure, "done"); closeErr != nil {
		t.Fatal(closeErr)
	}
	targetClosed = true
	waitForRequestCount(t, jw, 1, testTimeout)
	jw.maintenance(time.Hour)
	wantActive(1)
}
