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
	"strings"
	"sync"
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
	// These tests drive maintenance and dirty distribution explicitly.
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

func requireStatusDirty(t *testing.T, jw *Jaws, want uint32) {
	t.Helper()
	if got := jw.statusDirty.Swap(0); got != want {
		t.Fatalf("pending status metrics = %#x, want %#x", got, want)
	}
}

func renderStatusCount(t *testing.T, rq *Request, tagValue any, getter func() int) <-chan int {
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
	return valueCh
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

type gatedStatusResponseWriter struct {
	http.ResponseWriter
	started chan<- struct{}
	release <-chan struct{}
}

func (w *gatedStatusResponseWriter) WriteHeader(statusCode int) {
	if statusCode == http.StatusNoContent {
		w.started <- struct{}{}
		<-w.release
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func TestJaws_StatusMetricInvalidations(t *testing.T) {
	t.Run("unrelated endpoints", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		server := httptest.NewServer(jw)
		defer server.Close()
		for _, path := range []string{"/jaws/.ping", "/jaws/.tail/not-a-key", jw.serveJS.Name, jw.serveCSS.Name} {
			jw.ServeHTTP(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, path))
			requireStatusDirty(t, jw, 0)
		}
	})

	t.Run("Request registration and claim", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		initial := newStatusHTTPRequest("http://example.test", "/claim")
		rq := jw.NewRequest(httptest.NewRecorder(), initial)
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		if got := jw.UseRequest(rq.JawsKey, initial); got != rq {
			t.Fatalf("UseRequest() = %p, want %p", got, rq)
		}
		requireStatusDirty(t, jw, StatusMetricPendingRequests)

		jw.Close()
		closed := jw.NewRequest(httptest.NewRecorder(), initial)
		if closed.Context().Err() == nil {
			t.Fatal("NewRequest after Close has a live context")
		}
		requireStatusDirty(t, jw, 0)
	})

	t.Run("noscript", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		initial := newStatusHTTPRequest("http://example.test", "/noscript")
		rq := jw.NewRequest(httptest.NewRecorder(), initial)
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		if got := jw.UseRequest(rq.JawsKey, initial); got != rq {
			t.Fatalf("UseRequest() = %p, want %p", got, rq)
		}
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		rq.ServeHTTP(httptest.NewRecorder(), newStatusHTTPRequest("http://example.test", "/jaws/"+rq.JawsKeyString()+"/noscript"))
		requireStatusDirty(t, jw, StatusMetricActiveRequests|StatusMetricErrors)
		rq.ServeHTTP(httptest.NewRecorder(), newStatusHTTPRequest("http://example.test", "/jaws/"+rq.JawsKeyString()+"/noscript"))
		requireStatusDirty(t, jw, 0)
	})

	t.Run("session-backed noscript", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		sessionRequest := newStatusHTTPRequest("http://example.test", "/session")
		sess := jw.NewSession(httptest.NewRecorder(), sessionRequest)
		if sess == nil {
			t.Fatal("NewSession returned nil")
		}
		requireStatusDirty(t, jw, StatusMetricSessions)
		initial := newStatusHTTPRequest("http://example.test", "/noscript")
		initial.AddCookie(sess.Cookie())
		rq := jw.NewRequest(httptest.NewRecorder(), initial)
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		if got := jw.UseRequest(rq.JawsKey, initial); got != rq {
			t.Fatalf("UseRequest() = %p, want %p", got, rq)
		}
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		rq.ServeHTTP(httptest.NewRecorder(), newStatusHTTPRequest("http://example.test", "/jaws/"+rq.JawsKeyString()+"/noscript"))
		requireStatusDirty(t, jw, StatusMetricActiveRequests|StatusMetricActiveSessions|StatusMetricErrors)
	})

	t.Run("rejected Origin", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		server := httptest.NewServer(jw)
		defer server.Close()
		rq := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/rejected"))
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		header := http.Header{}
		header.Set("Origin", "https://evil.invalid")
		conn, response, err := websocket.Dial(t.Context(), server.URL+"/jaws/"+rq.JawsKeyString(), &websocket.DialOptions{HTTPHeader: header})
		if conn != nil {
			closeStatusWebSocket(t, conn)
			t.Fatal("wrong-origin WebSocket was accepted")
		}
		if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
			t.Fatalf("wrong-origin dial response = %v, err = %v", response, err)
		}
		if response.Body != nil {
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Error(closeErr)
			}
		}
		requireStatusDirty(t, jw, StatusMetricActiveRequests|StatusMetricPendingRequests|StatusMetricErrors)
	})

	t.Run("failed upgrade", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		server := httptest.NewServer(jw)
		defer server.Close()
		rq := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/failed"))
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/jaws/"+rq.JawsKeyString(), nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Origin", server.URL)
		request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		request.Header.Set("Sec-WebSocket-Version", "12")
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusBadRequest {
			t.Errorf("failed-upgrade status = %d, want %d", response.StatusCode, http.StatusBadRequest)
		}
		if err = response.Body.Close(); err != nil {
			t.Error(err)
		}
		requireStatusDirty(t, jw, StatusMetricActiveRequests|StatusMetricPendingRequests|StatusMetricErrors)
	})

	t.Run("accepted WebSocket", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		server := httptest.NewServer(jw)
		defer server.Close()
		rq := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/accepted"))
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		atConnect := make(chan uint32, 1)
		rq.SetConnectFn(func(*Request) error {
			atConnect <- jw.statusDirty.Load()
			return errors.New("connect rejected")
		})
		conn := dialJawsRequest(t, server.URL, rq)
		want := StatusMetricActiveRequests | StatusMetricPendingRequests | StatusMetricActiveSessions
		if got := awaitStatusValue(t, atConnect, "accepted WebSocket"); got != want {
			t.Fatalf("pending status metrics in ConnectFn = %#x, want %#x", got, want)
		}
		closeStatusWebSocket(t, conn)
		waitForRequestCount(t, jw, 0, testTimeout)
		requireStatusDirty(t, jw, want|StatusMetricErrors)
	})

	t.Run("Session registration", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		request := newStatusHTTPRequest("http://example.test", "/session")
		sess := jw.NewSession(httptest.NewRecorder(), request)
		if sess == nil {
			t.Fatal("NewSession returned nil")
		}
		requireStatusDirty(t, jw, StatusMetricSessions)
		sess.Close()
		requireStatusDirty(t, jw, StatusMetricSessions|StatusMetricActiveSessions)

		handler := jw.SessionMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		handler.ServeHTTP(httptest.NewRecorder(), newStatusHTTPRequest("http://example.test", "/middleware"))
		requireStatusDirty(t, jw, StatusMetricSessions)

		jw.Close()
		if got := jw.NewSession(httptest.NewRecorder(), request); got != nil {
			t.Fatalf("NewSession after Close = %p, want nil", got)
		}
		requireStatusDirty(t, jw, 0)
	})

	t.Run("AutoSession registration", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(jw *Jaws) { jw.AutoSession = true })
		server := httptest.NewServer(jw)
		defer server.Close()
		rq := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/auto-session"))
		requireStatusDirty(t, jw, StatusMetricPendingRequests)
		connected := make(chan *Session, 1)
		atConnect := make(chan uint32, 1)
		rq.SetConnectFn(func(rq *Request) error {
			connected <- rq.Session()
			atConnect <- jw.statusDirty.Load()
			return nil
		})
		conn := dialJawsRequest(t, server.URL, rq)
		defer closeStatusWebSocket(t, conn)
		if sess := awaitStatusValue(t, connected, "AutoSession WebSocket"); sess == nil {
			t.Fatal("AutoSession was not registered before ConnectFn")
		}
		want := StatusMetricActiveRequests | StatusMetricPendingRequests | StatusMetricSessions | StatusMetricActiveSessions
		if got := awaitStatusValue(t, atConnect, "AutoSession invalidation"); got != want {
			t.Fatalf("pending status metrics in ConnectFn = %#x, want %#x", got, want)
		}
	})

	t.Run("errors coalesce", func(t *testing.T) {
		jw := newStatusTestJaws(t, func(*Jaws) {})
		_ = jw.Log(errors.New("first"))
		_ = jw.Log(nil)
		_ = jw.Log(errors.New("second"))
		requireStatusDirty(t, jw, StatusMetricErrors)
	})
}

func TestJaws_ActiveCountTagsConvergeBetweenMaintenancePassesOnReconnect(t *testing.T) {
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

func TestJaws_ActiveCountTagsConvergeBetweenMaintenancePassesOnNoscript(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.StatusMetrics.Store(StatusMetricActiveRequests | StatusMetricActiveSessions)
	})
	startedCh := make(chan struct{}, 1)
	releaseCh := make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseCh) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/noscript") {
			w = &gatedStatusResponseWriter{ResponseWriter: w, started: startedCh, release: releaseCh}
		}
		jw.ServeHTTP(w, r)
	}))
	defer func() {
		release()
		server.Close()
	}()

	sess := jw.NewSession(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/session"))
	if sess == nil {
		t.Fatal("NewSession returned nil")
	}
	observer := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/observer"))
	observerConn := dialConnectedStatusRequest(t, server.URL, observer, "observer WebSocket")
	defer closeStatusWebSocket(t, observerConn)

	targetInitial := newStatusHTTPRequest(server.URL, "/target")
	targetInitial.AddCookie(sess.Cookie())
	target := jw.NewRequest(httptest.NewRecorder(), targetInitial)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.ActiveRequestCountTag(), jw.ActiveSessionCountTag())

	type responseResult struct {
		response *http.Response
		err      error
	}
	responseCh := make(chan responseResult, 1)
	go func() {
		response, err := server.Client().Get(server.URL + "/jaws/" + target.JawsKeyString() + "/noscript")
		responseCh <- responseResult{response: response, err: err}
	}()
	awaitStatusValue(t, startedCh, "running noscript Request")

	requestValues := renderStatusCount(t, observer, jw.ActiveRequestCountTag(), func() int {
		_, active := jw.RequestCounts()
		return active
	})
	sessionValues := renderStatusCount(t, observer, jw.ActiveSessionCountTag(), jw.ActiveSessionCount)
	requireStatusCount(t, requestValues, 2)
	requireStatusCount(t, sessionValues, 1)

	release()
	result := awaitStatusValue(t, responseCh, "noscript response")
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.response.StatusCode != http.StatusNoContent {
		t.Errorf("noscript status = %d, want %d", result.response.StatusCode, http.StatusNoContent)
	}
	if err := result.response.Body.Close(); err != nil {
		t.Error(err)
	}
	waitForRequestCount(t, jw, 1, testTimeout)
	if got := jw.ActiveSessionCount(); got != 0 {
		t.Fatalf("ActiveSessionCount() = %d, want 0", got)
	}

	jw.maintenance(time.Hour)
	dispatchStatusDirt(t, jw, 2)
	requireStatusCount(t, requestValues, 1)
	requireStatusCount(t, sessionValues, 0)
}

func TestJaws_SessionCountTagConvergesBetweenMaintenancePasses(t *testing.T) {
	jw := newStatusTestJaws(t, func(jw *Jaws) {
		jw.StatusMetrics.Store(StatusMetricSessions)
	})
	server := httptest.NewServer(jw)
	defer server.Close()

	first := jw.NewSession(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/first-session"))
	if first == nil {
		t.Fatal("NewSession returned nil")
	}
	observer := jw.NewRequest(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/observer"))
	observerConn := dialConnectedStatusRequest(t, server.URL, observer, "observer WebSocket")
	defer closeStatusWebSocket(t, observerConn)
	jw.maintenance(time.Hour)
	requireDirtyTags(t, jw, jw.SessionCountTag())

	second := jw.NewSession(httptest.NewRecorder(), newStatusHTTPRequest(server.URL, "/second-session"))
	if second == nil {
		t.Fatal("NewSession returned nil")
	}
	sessionValues := renderStatusCount(t, observer, jw.SessionCountTag(), jw.SessionCount)
	requireStatusCount(t, sessionValues, 2)
	first.Close()
	if got := jw.SessionCount(); got != 1 {
		t.Fatalf("SessionCount() = %d, want 1", got)
	}

	jw.maintenance(time.Hour)
	dispatchStatusDirt(t, jw, 1)
	requireStatusCount(t, sessionValues, 1)
}

func TestJaws_PendingRequestCountTagConvergesBetweenMaintenancePasses(t *testing.T) {
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
	release := sync.OnceFunc(func() { close(releaseCh) })
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
	defer func() {
		release()
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

	release()
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

	// Lifecycle marks survive a disable/enable pair that falls entirely between
	// maintenance passes, when the last-observed selection remains unchanged.
	jw.StatusMetrics.And(0)
	jw.newRequest(httptest.NewRequest(http.MethodGet, "/third", nil))
	_ = jw.Log(errors.New("disabled between passes"))
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

	// Drain the first-selection dirty mark before registering the observer Element.
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
