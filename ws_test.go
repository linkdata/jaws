package jaws

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
	"nhooyr.io/websocket"
)

type testServer struct {
	jw          *Jaws
	ctx         context.Context
	cancel      context.CancelFunc
	hr          *http.Request
	rr          *httptest.ResponseRecorder
	rq          *Request
	sess        *Session
	srv         *httptest.Server
	connectedCh chan struct{}
}

func newTestServer() (ts *testServer) {
	jw := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	rr := httptest.NewRecorder()
	hr := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	sess := jw.NewSession(rr, hr)
	rq := jw.NewRequest(hr)
	if rq != jw.UseRequest(rq.JawsKey, hr) {
		panic("UseRequest failed")
	}
	ts = &testServer{
		jw:          jw,
		ctx:         ctx,
		cancel:      cancel,
		hr:          hr,
		rr:          rr,
		rq:          rq,
		sess:        sess,
		connectedCh: make(chan struct{}),
	}
	rq.SetConnectFn(ts.connected)
	ts.srv = httptest.NewServer(ts)
	return
}

func (ts *testServer) connected(rq *Request) error {
	if rq == ts.rq {
		close(ts.connectedCh)
	}
	return nil
}

func (ts *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/jaws/") {
		jawsKey := JawsKeyValue(strings.TrimPrefix(r.URL.Path, "/jaws/"))
		if rq := ts.jw.UseRequest(jawsKey, r); rq != nil {
			rq.ServeHTTP(w, r)
			return
		}
	}
	ts.rq.ServeHTTP(w, r)
}

func (ts *testServer) Path() string {
	return "/jaws/" + ts.rq.JawsKeyString()
}

func (ts *testServer) Url() string {
	return ts.srv.URL + ts.Path()
}

func (ts *testServer) Close() {
	ts.cancel()
	ts.srv.Close()
	ts.jw.Close()
}

func TestWS_UpgradeRequired(t *testing.T) {
	jw := New()
	defer jw.Close()
	w := httptest.NewRecorder()
	hr := httptest.NewRequest("", "/", nil)
	rq := jw.NewRequest(hr)
	jw.UseRequest(rq.JawsKey, hr)
	req := httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	rq.ServeHTTP(w, req)
	if w.Code != http.StatusUpgradeRequired {
		t.Error(w.Code)
	}
}

func TestWS_ConnectFnFails(t *testing.T) {
	const nope = "nope"
	ts := newTestServer()
	defer ts.Close()
	ts.rq.SetConnectFn(func(_ *Request) error { return errors.New(nope) })

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	if conn != nil {
		defer conn.Close(websocket.StatusNormalClosure, "")
	}
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}
	mt, b, err := conn.Read(ts.ctx)
	if err != nil {
		t.Error(err)
	}
	if mt != websocket.MessageText {
		t.Error(mt)
	}
	if !strings.Contains(string(b), nope) {
		t.Error(string(b))
	}
}

func TestWS_NormalExchange(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	fooError := errors.New("this foo failed")

	gotCallCh := make(chan struct{})

	ts.rq.Register(("foo"), func(e *Element, evt what.What, val string) error {
		close(gotCallCh)
		return fooError
	})

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	msg := wsMsg{Jid: jidForTag(ts.rq, Tag("foo")), What: what.Input}
	ctx, cancel := context.WithTimeout(ts.ctx, testTimeout)
	defer cancel()

	err = conn.Write(ctx, websocket.MessageText, msg.Append(nil))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-th.C:
		th.Timeout()
	case <-gotCallCh:
	}

	mt, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mt != websocket.MessageText {
		t.Error(mt)
	}
	var m2 wsMsg
	m2.FillAlert(fooError)
	if !bytes.Equal(b, m2.Append(nil)) {
		t.Error(b)
	}
}

func TestReader_RespectsContextDone(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	msg := wsMsg{Jid: Jid(1234), What: what.Input}
	doneCh := make(chan struct{})
	inCh := make(chan wsMsg)
	client, server := Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	go func() {
		defer close(doneCh)
		wsReader(ts.ctx, nil, ts.jw.Done(), inCh, server)
	}()

	client.Write(ctx, websocket.MessageText, []byte(msg.Format()))

	// wsReader should now be blocked trying to send the decoded message
	select {
	case <-doneCh:
		t.Error("did not block")
	case <-time.NewTimer(time.Millisecond).C:
	}

	ts.cancel()

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
	}
}

func TestReader_RespectsJawsDone(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	doneCh := make(chan struct{})
	inCh := make(chan wsMsg)
	client, server := Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	go func() {
		defer close(doneCh)
		wsReader(ts.ctx, nil, ts.jw.Done(), inCh, server)
	}()

	ts.jw.Close()
	msg := wsMsg{Jid: Jid(1234), What: what.Input}
	err := client.Write(ctx, websocket.MessageText, []byte(msg.Format()))
	if err != nil {
		t.Error(err)
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
	}
}

func TestWriter_SendsThePayload(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	outCh := make(chan string)
	defer close(outCh)
	client, server := Pipe()

	go wsWriter(ts.ctx, nil, ts.jw.Done(), outCh, server)

	var mt websocket.MessageType
	var b []byte
	var err error
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		mt, b, err = client.Read(ts.ctx)
		ts.cancel()
	}()

	msg := wsMsg{Jid: Jid(1234)}
	select {
	case <-th.C:
		th.Timeout()
	case outCh <- msg.Format():
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
	}

	if err != nil {
		t.Error(err)
	}
	if mt != websocket.MessageText {
		t.Error(mt)
	}
	if string(b) != msg.Format() {
		t.Error(string(b))
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-client.CloseRead(ts.ctx).Done():
	}
}

func TestWriter_RespectsContext(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	doneCh := make(chan struct{})
	outCh := make(chan string)
	defer close(outCh)
	client, server := Pipe()
	client.CloseRead(context.Background())

	go func() {
		defer close(doneCh)
		wsWriter(ts.ctx, nil, ts.jw.Done(), outCh, server)
	}()

	ts.cancel()

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
		return
	}
}

func TestWriter_RespectsJawsDone(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	doneCh := make(chan struct{})
	outCh := make(chan string)
	defer close(outCh)
	client, server := Pipe()
	client.CloseRead(ts.ctx)

	go func() {
		defer close(doneCh)
		wsWriter(ts.ctx, nil, ts.jw.Done(), outCh, server)
	}()

	ts.jw.Close()

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
	}
}

func TestWriter_RespectsOutboundClosed(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	doneCh := make(chan struct{})
	outCh := make(chan string)
	client, server := Pipe()
	client.CloseRead(ts.ctx)

	go func() {
		defer close(doneCh)
		wsWriter(ts.ctx, nil, ts.jw.Done(), outCh, server)
	}()

	close(outCh)

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
	}

	if err := ts.rq.Context().Err(); err != nil {
		t.Error(err)
	}
}

func TestWriter_ReportsError(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	doneCh := make(chan struct{})
	outCh := make(chan string)
	client, server := Pipe()
	client.CloseRead(ts.ctx)
	server.Close(websocket.StatusNormalClosure, "")

	go func() {
		defer close(doneCh)
		wsWriter(ts.rq.ctx, ts.rq.cancelFn, ts.jw.Done(), outCh, server)
	}()

	msg := wsMsg{Jid: Jid(1234)}
	select {
	case <-th.C:
		th.Timeout()
	case outCh <- msg.Format():
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
	}

	err := context.Cause(ts.rq.Context())
	if !strings.Contains(err.Error(), "WebSocket closed") {
		t.Error(err)
	}
}

func TestReader_ReportsError(t *testing.T) {
	th := newTestHelper(t)
	ts := newTestServer()
	defer ts.Close()

	doneCh := make(chan struct{})
	inCh := make(chan wsMsg)
	client, server := Pipe()
	client.CloseRead(ts.ctx)
	server.Close(websocket.StatusNormalClosure, "")

	go func() {
		defer close(doneCh)
		wsReader(ts.rq.ctx, ts.rq.cancelFn, ts.jw.Done(), inCh, server)
	}()

	msg := wsMsg{Jid: Jid(1234), What: what.Input}
	err := client.Write(ts.ctx, websocket.MessageText, []byte(msg.Format()))
	if err == nil {
		t.Fatal("expected error")
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-doneCh:
	}

	err = context.Cause(ts.rq.Context())
	if !strings.Contains(err.Error(), "WebSocket closed") {
		t.Error(err)
	}
}

// adapted from nhooyr.io/websocket/internal/test/wstest.Pipe

func Pipe() (clientConn, serverConn *websocket.Conn) {
	dialOpts := &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: fakeTransport{
				h: func(w http.ResponseWriter, r *http.Request) {
					serverConn, _ = websocket.Accept(w, r, nil)
				},
			},
		},
	}

	clientConn, _, _ = websocket.Dial(context.Background(), "ws://localhost", dialOpts)
	return clientConn, serverConn
}

type fakeTransport struct {
	h http.HandlerFunc
}

func (t fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clientConn, serverConn := net.Pipe()

	hj := testHijacker{
		ResponseRecorder: httptest.NewRecorder(),
		serverConn:       serverConn,
	}

	t.h.ServeHTTP(hj, r)

	resp := hj.ResponseRecorder.Result()
	if resp.StatusCode == http.StatusSwitchingProtocols {
		resp.Body = clientConn
	}
	return resp, nil
}

type testHijacker struct {
	*httptest.ResponseRecorder
	serverConn net.Conn
}

var _ http.Hijacker = testHijacker{}

func (hj testHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return hj.serverConn, bufio.NewReadWriter(bufio.NewReader(hj.serverConn), bufio.NewWriter(hj.serverConn)), nil
}
