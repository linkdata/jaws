package jaws

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func newStatusTestJaws(t *testing.T, configure func(*Jaws)) *Jaws {
	t.Helper()
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// These tests drive maintenance and dirty distribution explicitly. Their real
	// loopback WebSockets keep them outside testing/synctest.
	jw.updateTicker.Stop()
	jw.newMaintenanceTicker = func(time.Duration) *time.Ticker {
		ticker := time.NewTicker(time.Hour)
		ticker.Stop()
		return ticker
	}
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
	jw.clearDirtLocked()
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

func awaitStatusValue[T any](t *testing.T, ch <-chan T, description string) (value T) {
	t.Helper()
	select {
	case value = <-ch:
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for %s", description)
	}
	return
}

func dialConnectedStatusRequest(t *testing.T, serverURL string, rq *Request, description string) (conn *websocket.Conn) {
	t.Helper()
	connected := make(chan struct{})
	rq.SetConnectFn(func(*Request) error {
		close(connected)
		return nil
	})
	conn = dialJawsRequest(t, serverURL, rq)
	awaitStatusValue(t, connected, description)
	return
}

func closeStatusWebSocket(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.CloseNow(); err != nil {
		t.Error(err)
	}
}

func statusGenerations(jw *Jaws) (registered, accepted uint64) {
	jw.mu.RLock()
	registered = jw.registeredRequestGen
	accepted = jw.acceptedWebSocketGen
	jw.mu.RUnlock()
	return
}

func requireStatusGenerations(t *testing.T, jw *Jaws, registered, accepted uint64) {
	t.Helper()
	gotRegistered, gotAccepted := statusGenerations(jw)
	if gotRegistered != registered || gotAccepted != accepted {
		t.Fatalf("status generations = (%d, %d), want (%d, %d)", gotRegistered, gotAccepted, registered, accepted)
	}
}

func renderStatusCount(t *testing.T, rq *Request, tagValue any, getter func() int) (values <-chan int) {
	t.Helper()
	valueCh := make(chan int, 2)
	statusUI := &testUi{
		renderFn: func(elem *Element, w io.Writer, _ []any) (err error) {
			elem.Tag(tagValue)
			value := getter()
			valueCh <- value
			_, err = fmt.Fprintf(w, `<span id="%s">%d</span>`, elem.Jid(), value)
			return
		},
		updateFn: func(elem *Element) {
			value := getter()
			innerHTML := template.HTML(strconv.Itoa(value)) // #nosec G203 -- a decimal integer contains no HTML syntax.
			elem.SetInner(innerHTML)
			valueCh <- value
		},
	}
	if err := rq.Writer(io.Discard).UI(statusUI); err != nil {
		t.Fatal(err)
	}
	values = valueCh
	return
}

func dispatchStatusDirt(t *testing.T, jw *Jaws, want int) {
	t.Helper()
	if got := jw.distributeDirt(); got != want {
		t.Fatalf("distributed status tags = %d, want %d", got, want)
	}
	jw.Broadcast(wire.Message{What: what.Update})
}

func requireStatusCount(t *testing.T, values <-chan int, want int) {
	t.Helper()
	if got := awaitStatusValue(t, values, fmt.Sprintf("status count %d", want)); got != want {
		t.Fatalf("rendered status count = %d, want %d", got, want)
	}
}

func newStatusHTTPRequest(serverURL, path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, serverURL+path, nil)
	r.RemoteAddr = "127.0.0.1:1"
	return r
}

func TestJaws_StatusMetricGenerations(t *testing.T) {
	jw := newStatusTestJaws(t, func(*Jaws) {})
	server := httptest.NewServer(jw)
	defer server.Close()
	requireStatusGenerations(t, jw, 0, 0)

	for _, path := range []string{"/jaws/.ping", "/jaws/.tail/not-a-key", jw.serveJS.Name, jw.serveCSS.Name} {
		jw.ServeHTTP(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, path))
		requireStatusGenerations(t, jw, 0, 0)
	}

	claimInitial := newStatusHTTPRequest(server.URL, "/claim")
	claim := jw.NewRequest(httptest.NewRecorder(), claimInitial)
	requireStatusGenerations(t, jw, 1, 0)
	if got := jw.UseRequest(claim.JawsKey, claimInitial); got != claim {
		t.Fatalf("UseRequest() = %p, want %p", got, claim)
	}
	requireStatusGenerations(t, jw, 1, 0)
	claim.ServeHTTP(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/jaws/"+claim.JawsKeyString()+"/noscript"))
	requireStatusGenerations(t, jw, 1, 0)

	rejected := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/rejected"))
	requireStatusGenerations(t, jw, 2, 0)
	rejectedHeader := http.Header{}
	rejectedHeader.Set("Origin", "https://evil.invalid")
	rejectedConn, rejectedResponse, err := websocket.Dial(t.Context(), server.URL+"/jaws/"+rejected.JawsKeyString(), &websocket.DialOptions{HTTPHeader: rejectedHeader})
	if rejectedConn != nil {
		closeStatusWebSocket(t, rejectedConn)
		t.Fatal("wrong-origin WebSocket was accepted")
	}
	if err == nil || rejectedResponse == nil || rejectedResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong-origin dial response = %v, err = %v", rejectedResponse, err)
	}
	if rejectedResponse.Body != nil {
		if closeErr := rejectedResponse.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}
	requireStatusGenerations(t, jw, 2, 0)

	failed := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/failed"))
	requireStatusGenerations(t, jw, 3, 0)
	failedRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/jaws/"+failed.JawsKeyString(), nil)
	if err != nil {
		t.Fatal(err)
	}
	failedRequest.Header.Set("Connection", "Upgrade")
	failedRequest.Header.Set("Upgrade", "websocket")
	failedRequest.Header.Set("Origin", server.URL)
	failedRequest.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	failedRequest.Header.Set("Sec-WebSocket-Version", "12")
	failedResponse, err := server.Client().Do(failedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if failedResponse.StatusCode != http.StatusBadRequest {
		t.Errorf("failed-upgrade status = %d, want %d", failedResponse.StatusCode, http.StatusBadRequest)
	}
	if err = failedResponse.Body.Close(); err != nil {
		t.Error(err)
	}
	requireStatusGenerations(t, jw, 3, 0)

	connectErr := errors.New("connect rejected")
	accepted := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/accepted"))
	requireStatusGenerations(t, jw, 4, 0)
	acceptedAtConnect := make(chan uint64, 1)
	accepted.SetConnectFn(func(*Request) error {
		_, generation := statusGenerations(jw)
		acceptedAtConnect <- generation
		return connectErr
	})
	acceptedConn := dialJawsRequest(t, server.URL, accepted)
	if generation := awaitStatusValue(t, acceptedAtConnect, "accepted WebSocket"); generation != 1 {
		t.Fatalf("accepted WebSocket generation in ConnectFn = %d, want 1", generation)
	}
	requireStatusGenerations(t, jw, 4, 1)
	closeStatusWebSocket(t, acceptedConn)
	waitForRequestCount(t, jw, 0, testTimeout)
	requireStatusGenerations(t, jw, 4, 1)

	jw.Close()
	closedRequest := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/closed"))
	if closedRequest.Context().Err() == nil {
		t.Fatal("NewRequest after Close has a live context")
	}
	requireStatusGenerations(t, jw, 4, 1)
}

func TestJaws_ActiveCountTagsConvergeAfterBetweenSampleReconnect(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.StatusMetrics.Store(StatusMetricActiveRequests | StatusMetricActiveSessions)
	})
	server := httptest.NewServer(jw)
	defer server.Close()

	sess := jw.NewSession(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/session"))
	if sess == nil {
		t.Fatal("NewSession returned nil")
	}
	oldInitial := newStatusHTTPRequest(server.URL, "/old")
	oldInitial.AddCookie(sess.Cookie())
	old := jw.NewRequest(httptest.NewRecorder(), oldInitial)
	oldConn := dialConnectedStatusRequest(t, server.URL, old, "old WebSocket")
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveRequestCountTag(), jw.ActiveSessionCountTag())

	closeStatusWebSocket(t, oldConn)
	waitForRequestCount(t, jw, 0, testTimeout)
	if got := jw.ActiveSessionCount(); got != 0 {
		t.Fatalf("ActiveSessionCount() = %d, want 0", got)
	}

	replacementInitial := newStatusHTTPRequest(server.URL, "/replacement")
	replacementInitial.AddCookie(sess.Cookie())
	replacement := jw.NewRequest(httptest.NewRecorder(), replacementInitial)
	_, active := jw.RequestCounts()
	if active != 0 {
		t.Fatalf("active Request count = %d, want 0", active)
	}
	requestValues := renderStatusCount(t, replacement, jw.ActiveRequestCountTag(), func() int {
		_, active := jw.RequestCounts()
		return active
	})
	sessionValues := renderStatusCount(t, replacement, jw.ActiveSessionCountTag(), jw.ActiveSessionCount)
	requireStatusCount(t, requestValues, 0)
	requireStatusCount(t, sessionValues, 0)

	replacementConn := dialConnectedStatusRequest(t, server.URL, replacement, "replacement WebSocket")
	defer closeStatusWebSocket(t, replacementConn)
	_, active = jw.RequestCounts()
	if active != 1 {
		t.Fatalf("active Request count = %d, want 1", active)
	}
	if got := jw.ActiveSessionCount(); got != 1 {
		t.Fatalf("ActiveSessionCount() = %d, want 1", got)
	}

	jw.maintenance(time.Hour)
	dispatchStatusDirt(t, jw, 2)
	requireStatusCount(t, requestValues, 1)
	requireStatusCount(t, sessionValues, 1)
}

func TestJaws_PendingRequestCountTagConvergesAfterBetweenSampleLifecycle(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.StatusMetrics.Store(StatusMetricPendingRequests)
	})
	server := httptest.NewServer(jw)
	defer server.Close()

	observer := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/observer"))
	observerConn := dialConnectedStatusRequest(t, server.URL, observer, "observer WebSocket")
	defer closeStatusWebSocket(t, observerConn)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.PendingRequestCountTag())

	target := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/target"))
	// A custom client may connect while the initial HTTP renderer continues. Render
	// the status Element through that supported overlap while target is pending.
	pendingValues := renderStatusCount(t, observer, jw.PendingRequestCountTag(), jw.Pending)
	requireStatusCount(t, pendingValues, 1)

	response, err := server.Client().Get(server.URL + "/jaws/" + target.JawsKeyString() + "/noscript")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Errorf("noscript status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if err = response.Body.Close(); err != nil {
		t.Error(err)
	}
	if got := jw.Pending(); got != 0 {
		t.Fatalf("Pending() = %d, want 0", got)
	}

	jw.maintenance(time.Hour)
	dispatchStatusDirt(t, jw, 1)
	requireStatusCount(t, pendingValues, 0)
}

func TestJaws_PendingRequestCountTagInvalidatedAfterAcceptedWebSocket(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.StatusMetrics.Store(StatusMetricPendingRequests)
	})
	claimedCh := make(chan *Request, 1)
	releaseCh := make(chan struct{})
	var target *Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rq := jw.UseRequest(target.JawsKey, r)
		claimedCh <- rq
		<-releaseCh
		if rq == nil {
			http.NotFound(w, r)
			return
		}
		rq.ServeHTTP(w, r)
	}))
	released := false
	defer func() {
		if !released {
			close(releaseCh)
		}
		server.Close()
	}()

	target = jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/target"))
	connectedCh := make(chan struct{})
	target.SetConnectFn(func(*Request) error {
		close(connectedCh)
		return nil
	})
	type dialResult struct {
		conn     *websocket.Conn
		response *http.Response
		err      error
	}
	dialCh := make(chan dialResult, 1)
	go func() {
		header := http.Header{}
		header.Set("Origin", server.URL)
		conn, response, err := websocket.Dial(t.Context(), server.URL+"/jaws/"+target.JawsKeyString(), &websocket.DialOptions{HTTPHeader: header})
		dialCh <- dialResult{conn: conn, response: response, err: err}
	}()

	if claimed := awaitStatusValue(t, claimedCh, "Request claim"); claimed != target {
		t.Fatalf("UseRequest() = %p, want %p", claimed, target)
	}
	if got := jw.Pending(); got != 0 {
		t.Fatalf("Pending() after claim = %d, want 0", got)
	}
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.PendingRequestCountTag())

	close(releaseCh)
	released = true
	result := awaitStatusValue(t, dialCh, "WebSocket acceptance")
	if result.err != nil {
		if result.response != nil && result.response.Body != nil {
			if closeErr := result.response.Body.Close(); closeErr != nil {
				t.Error(closeErr)
			}
		}
		t.Fatal(result.err)
	}
	defer closeStatusWebSocket(t, result.conn)
	awaitStatusValue(t, connectedCh, "accepted WebSocket")

	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.PendingRequestCountTag())
}

func TestJaws_StatusMetrics(t *testing.T) {
	if got, want := StatusMetricAll, StatusMetricErrors<<1-1; got != want {
		t.Fatalf("StatusMetricAll = %#x, want %#x", got, want)
	}

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
	requireDirtyTags(
		t, jw,
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
