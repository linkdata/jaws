package ui

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

const handlerWebSocketTestTimeout = 5 * time.Second

type handlerWebSocketServer struct {
	server   *httptest.Server
	requests chan *jaws.Request
}

func newHandlerWebSocketServer(t *testing.T, source string, dot any, funcs template.FuncMap) (ts *handlerWebSocketServer) {
	t.Helper()

	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	requests := make(chan *jaws.Request, 64)
	templateFuncs := template.FuncMap{
		"captureHandlerRequest": func(with With) string {
			requests <- with.RequestWriter.Request
			return ""
		},
	}
	for name, fn := range funcs {
		templateFuncs[name] = fn
	}
	tmpl, err := template.New("page").Funcs(templateFuncs).Parse(source)
	if err != nil {
		jw.Close()
		t.Fatal(err)
	}
	if err = jw.AddTemplateLookuper(tmpl); err != nil {
		jw.Close()
		t.Fatal(err)
	}

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		jw.Serve()
	}()

	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)
	mux.Handle("GET /", Handler(jw, "page", dot))
	server := httptest.NewServer(mux)
	ts = &handlerWebSocketServer{server: server, requests: requests}
	t.Cleanup(func() {
		jw.Close()
		server.Close()
		<-serveDone
	})
	return
}

func (ts *handlerWebSocketServer) get(ctx context.Context) (body string, err error) {
	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, ts.server.URL+"/", nil); err == nil {
		var resp *http.Response
		if resp, err = ts.server.Client().Do(req); err == nil {
			var data []byte
			var readErr error
			data, readErr = io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			err = errors.Join(readErr, closeErr)
			body = string(data)
			if err == nil && resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("GET status = %d, want %d; body %q", resp.StatusCode, http.StatusOK, body)
			}
		}
	}
	return
}

func (ts *handlerWebSocketServer) dial(ctx context.Context, rq *jaws.Request) (conn *websocket.Conn, err error) {
	header := http.Header{}
	header.Set("Origin", ts.server.URL)
	var resp *http.Response
	wsURL := "ws" + strings.TrimPrefix(ts.server.URL, "http") + "/jaws/" + rq.JawsKeyString()
	conn, resp, err = websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err == nil {
		if resp == nil {
			err = errors.New("WebSocket handshake returned no response")
		} else if resp.StatusCode != http.StatusSwitchingProtocols {
			err = fmt.Errorf("WebSocket status = %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
		}
	}
	if err != nil && conn != nil {
		err = errors.Join(err, conn.CloseNow())
		conn = nil
	}
	return
}

func receiveHandlerWebSocketValue[T any](t *testing.T, ctx context.Context, ch <-chan T, description string) (value T) {
	t.Helper()
	select {
	case value = <-ch:
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", description, context.Cause(ctx))
	}
	return
}

func waitHandlerWebSocketDone(t *testing.T, ctx context.Context, done <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", description, context.Cause(ctx))
	}
}

func closeHandlerWebSocket(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if conn != nil {
		if err := conn.CloseNow(); err != nil {
			t.Errorf("closing WebSocket: %v", err)
		}
	}
}

type connectHandlerRecorder struct {
	mu         sync.Mutex
	calls      []*jaws.Request
	connectErr error
	called     chan *jaws.Request
}

func (rec *connectHandlerRecorder) JawsConnect(rq *jaws.Request) (err error) {
	rec.mu.Lock()
	rec.calls = append(rec.calls, rq)
	err = rec.connectErr
	rec.mu.Unlock()
	if rec.called != nil {
		rec.called <- rq
	}
	return
}

func (rec *connectHandlerRecorder) snapshot() (calls []*jaws.Request) {
	rec.mu.Lock()
	calls = append(calls, rec.calls...)
	rec.mu.Unlock()
	return
}

type connectBeforeClickDot struct {
	mu             sync.Mutex
	order          []string
	connectEntered chan struct{}
	releaseConnect <-chan struct{}
	clickCalled    chan struct{}
	enterOnce      sync.Once
	clickOnce      sync.Once
}

func (dot *connectBeforeClickDot) JawsConnect(*jaws.Request) error {
	dot.mu.Lock()
	dot.order = append(dot.order, "connect start")
	dot.mu.Unlock()
	dot.enterOnce.Do(func() { close(dot.connectEntered) })
	<-dot.releaseConnect
	dot.mu.Lock()
	dot.order = append(dot.order, "connect return")
	dot.mu.Unlock()
	return nil
}

func (dot *connectBeforeClickDot) JawsClick(*jaws.Element, jaws.Click) error {
	dot.mu.Lock()
	dot.order = append(dot.order, "click")
	dot.mu.Unlock()
	dot.clickOnce.Do(func() { close(dot.clickCalled) })
	return nil
}

func (dot *connectBeforeClickDot) snapshot() (order []string) {
	dot.mu.Lock()
	order = append(order, dot.order...)
	dot.mu.Unlock()
	return
}

func TestHandler_DotWithoutConnectHandlerUnchanged(t *testing.T) {
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}hello {{.Dot}}`, "world", nil)
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	body, err := ts.get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if body != "hello world" {
		t.Fatalf("body = %q, want %q", body, "hello world")
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	if rq.GetConnectFn() != nil {
		t.Fatal("plain page dot installed a ConnectFn")
	}
}

func TestHandler_GETDoesNotInvokeConnectHandler(t *testing.T) {
	dot := new(connectHandlerRecorder)
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}`, dot, nil)
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	if _, err := ts.get(ctx); err != nil {
		t.Fatal(err)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	if calls := dot.snapshot(); len(calls) != 0 {
		t.Fatalf("JawsConnect calls after GET = %v, want none", calls)
	}
	if rq.GetConnectFn() == nil {
		t.Fatal("GET did not install the page dot ConnectFn")
	}
}

func TestHandler_WebSocketInvokesConnectHandlerOnceForSameRequest(t *testing.T) {
	dot := &connectHandlerRecorder{called: make(chan *jaws.Request, 2)}
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}`, dot, nil)
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	if _, err := ts.get(ctx); err != nil {
		t.Fatal(err)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	rqCtx := rq.Context()
	conn, err := ts.dial(ctx, rq)
	if err != nil {
		t.Fatal(err)
	}
	if got := receiveHandlerWebSocketValue(t, ctx, dot.called, "JawsConnect call"); got != rq {
		t.Fatalf("JawsConnect Request = %p, want page Request %p", got, rq)
	}
	closeHandlerWebSocket(t, conn)
	waitHandlerWebSocketDone(t, ctx, rqCtx.Done(), "Request shutdown")

	calls := dot.snapshot()
	if len(calls) != 1 || calls[0] != rq {
		t.Fatalf("JawsConnect calls = %v, want [%p]", calls, rq)
	}
}

func TestHandler_ConnectHandlerRunsBeforeBrowserMessages(t *testing.T) {
	releaseConnect := make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseConnect) })
	defer release()
	dot := &connectBeforeClickDot{
		connectEntered: make(chan struct{}),
		releaseConnect: releaseConnect,
		clickCalled:    make(chan struct{}),
	}
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}{{$.Button "run" .Dot}}`, dot, nil)
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	if _, err := ts.get(ctx); err != nil {
		t.Fatal(err)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	elems := rq.GetElements(dot)
	if len(elems) != 1 {
		t.Fatalf("button Elements = %d, want 1", len(elems))
	}
	conn, err := ts.dial(ctx, rq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { closeHandlerWebSocket(t, conn) }()
	waitHandlerWebSocketDone(t, ctx, dot.connectEntered, "JawsConnect entry")

	click := wire.WsMsg{Jid: elems[0].Jid(), What: what.Click, Data: "0 0 0 run"}
	if err = conn.Write(ctx, websocket.MessageText, click.Append(nil)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-dot.clickCalled:
		t.Fatal("browser click ran while JawsConnect was blocked")
	default:
	}
	release()
	waitHandlerWebSocketDone(t, ctx, dot.clickCalled, "browser click")

	want := []string{"connect start", "connect return", "click"}
	if got := dot.snapshot(); !slices.Equal(got, want) {
		t.Fatalf("callback order = %v, want %v", got, want)
	}
}

func TestHandler_ConnectHandlerErrorClosesWebSocket(t *testing.T) {
	connectErr := errors.New("connect rejected")
	dot := &connectHandlerRecorder{
		connectErr: connectErr,
		called:     make(chan *jaws.Request, 2),
	}
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}`, dot, nil)
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	if _, err := ts.get(ctx); err != nil {
		t.Fatal(err)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	rqCtx := rq.Context()
	conn, err := ts.dial(ctx, rq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { closeHandlerWebSocket(t, conn) }()
	if got := receiveHandlerWebSocketValue(t, ctx, dot.called, "JawsConnect call"); got != rq {
		t.Fatalf("JawsConnect Request = %p, want page Request %p", got, rq)
	}
	waitHandlerWebSocketDone(t, ctx, rqCtx.Done(), "failed Request shutdown")
	if !errors.Is(context.Cause(rqCtx), connectErr) {
		t.Fatalf("Request cause = %v, want %v", context.Cause(rqCtx), connectErr)
	}
	if _, _, err = conn.Read(ctx); err == nil {
		t.Fatal("ConnectHandler error left WebSocket open")
	} else if ctx.Err() != nil {
		t.Fatalf("WebSocket remained open until timeout: %v", context.Cause(ctx))
	}
}

func TestHandler_ConnectHandlerInstalledBeforeTemplateExecution(t *testing.T) {
	type observation struct {
		rq        *jaws.Request
		installed bool
	}
	observed := make(chan observation, 1)
	dot := new(connectHandlerRecorder)
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{exposeHandlerRequestKey $}}`, dot, template.FuncMap{
		"exposeHandlerRequestKey": func(with With) string {
			rq := with.RequestWriter.Request
			observed <- observation{rq: rq, installed: rq.GetConnectFn() != nil}
			return rq.JawsKeyString()
		},
	})
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	body, err := ts.get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	got := receiveHandlerWebSocketValue(t, ctx, observed, "template observation")
	if got.rq != rq {
		t.Fatalf("template Request = %p, want page Request %p", got.rq, rq)
	}
	if !got.installed {
		t.Fatal("template exposed the Request key before ConnectFn was installed")
	}
	if body != rq.JawsKeyString() {
		t.Fatalf("exposed key = %q, want %q", body, rq.JawsKeyString())
	}
}

func TestHandler_NestedTemplateConnectHandlerIgnored(t *testing.T) {
	nested := &connectHandlerRecorder{called: make(chan *jaws.Request, 2)}
	page := struct {
		Nested *connectHandlerRecorder
	}{Nested: nested}
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}{{$.Template "div" "nested" .Dot.Nested}}{{define "nested"}}nested{{end}}`, page, nil)
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	if _, err := ts.get(ctx); err != nil {
		t.Fatal(err)
	}
	rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
	rqCtx := rq.Context()
	if rq.GetConnectFn() != nil {
		t.Fatal("nested Template dot installed the Request ConnectFn")
	}
	conn, err := ts.dial(ctx, rq)
	if err != nil {
		t.Fatal(err)
	}
	closeHandlerWebSocket(t, conn)
	waitHandlerWebSocketDone(t, ctx, rqCtx.Done(), "Request shutdown")
	if calls := nested.snapshot(); len(calls) != 0 {
		t.Fatalf("nested JawsConnect calls = %v, want none", calls)
	}
}

func TestHandler_SharedHandlerSupportsConcurrentRequests(t *testing.T) {
	const requestCount = 8
	dot := &connectHandlerRecorder{called: make(chan *jaws.Request, requestCount*2)}
	ts := newHandlerWebSocketServer(t, `{{captureHandlerRequest $}}{{$.HeadHTML}}`, dot, nil)
	ctx, cancel := context.WithTimeout(t.Context(), handlerWebSocketTestTimeout)
	defer cancel()

	getResults := make(chan error, requestCount)
	startGET := make(chan struct{})
	for range requestCount {
		go func() {
			<-startGET
			_, err := ts.get(ctx)
			getResults <- err
		}()
	}
	close(startGET)
	for range requestCount {
		if err := receiveHandlerWebSocketValue(t, ctx, getResults, "concurrent GET"); err != nil {
			t.Fatal(err)
		}
	}

	requests := make([]*jaws.Request, 0, requestCount)
	requestSet := make(map[*jaws.Request]struct{}, requestCount)
	requestContexts := make([]context.Context, 0, requestCount)
	for range requestCount {
		rq := receiveHandlerWebSocketValue(t, ctx, ts.requests, "page Request")
		if _, duplicate := requestSet[rq]; duplicate {
			t.Fatalf("duplicate page Request %p", rq)
		}
		requestSet[rq] = struct{}{}
		requests = append(requests, rq)
		requestContexts = append(requestContexts, rq.Context())
	}

	type dialResult struct {
		conn *websocket.Conn
		err  error
	}
	dialResults := make(chan dialResult, requestCount)
	startDial := make(chan struct{})
	for _, rq := range requests {
		go func() {
			<-startDial
			conn, err := ts.dial(ctx, rq)
			dialResults <- dialResult{conn: conn, err: err}
		}()
	}
	close(startDial)

	conns := make([]*websocket.Conn, 0, requestCount)
	defer func() {
		for _, conn := range conns {
			closeHandlerWebSocket(t, conn)
		}
	}()
	var dialErr error
	for range requestCount {
		result := receiveHandlerWebSocketValue(t, ctx, dialResults, "concurrent WebSocket dial")
		if result.conn != nil {
			conns = append(conns, result.conn)
		}
		dialErr = errors.Join(dialErr, result.err)
	}
	if dialErr != nil {
		t.Fatal(dialErr)
	}

	calledSet := make(map[*jaws.Request]struct{}, requestCount)
	for range requestCount {
		rq := receiveHandlerWebSocketValue(t, ctx, dot.called, "JawsConnect call")
		if _, ok := requestSet[rq]; !ok {
			t.Errorf("JawsConnect received unknown Request %p", rq)
		}
		if _, duplicate := calledSet[rq]; duplicate {
			t.Errorf("JawsConnect called more than once for Request %p", rq)
		}
		calledSet[rq] = struct{}{}
	}
	if calls := dot.snapshot(); len(calls) != requestCount {
		t.Fatalf("JawsConnect calls = %d, want %d", len(calls), requestCount)
	}

	for _, conn := range conns {
		closeHandlerWebSocket(t, conn)
	}
	conns = nil
	for i, rqCtx := range requestContexts {
		waitHandlerWebSocketDone(t, ctx, rqCtx.Done(), fmt.Sprintf("Request %d shutdown", i))
	}
}
