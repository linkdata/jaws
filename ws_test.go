package jaws

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
	"nhooyr.io/websocket"
)

type testServer struct {
	is          *is.I
	jw          *Jaws
	ctx         context.Context
	cancel      context.CancelFunc
	hr          *http.Request
	rq          *Request
	sess        *Session
	srv         *httptest.Server
	connectedCh chan struct{}
}

func newTestServer(is *is.I) (ts *testServer) {
	jw := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	hr := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	sess := jw.NewSession(nil, hr)
	rq := jw.NewRequest(hr)
	is.Equal(rq, jw.UseRequest(rq.JawsKey, hr))
	ts = &testServer{
		is:          is,
		jw:          jw,
		ctx:         ctx,
		cancel:      cancel,
		hr:          hr,
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
	is := is.New(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)

	req := httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	w := httptest.NewRecorder()
	rq.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusUpgradeRequired)
}

func TestWS_ConnectFnFails(t *testing.T) {
	const nope = "nope"
	is := is.New(t)
	ts := newTestServer(is)
	defer ts.Close()
	ts.rq.SetConnectFn(func(_ *Request) error { return errors.New(nope) })

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusSwitchingProtocols)
	defer conn.Close(websocket.StatusNormalClosure, "")
	mt, b, err := conn.Read(ts.ctx)
	is.NoErr(err)
	is.Equal(mt, websocket.MessageText)
	is.True(strings.Contains(string(b), nope))
}

func TestWS_NormalExchange(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
	defer ts.Close()

	fooError := errors.New("this foo failed")

	gotCallCh := make(chan struct{})

	ts.rq.Register(("foo"), func(e *Element, evt what.What, val string) (bool, error) {
		close(gotCallCh)
		return false, fooError
	})

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusSwitchingProtocols)
	defer conn.Close(websocket.StatusNormalClosure, "")

	msg := wsMsg{Jid: jidForTag(ts.rq, Tag("foo")), What: what.Input}
	ctx, cancel := context.WithTimeout(ts.ctx, time.Second*3)
	defer cancel()

	err = conn.Write(ctx, websocket.MessageText, msg.Append(nil))
	is.NoErr(err)
	select {
	case <-time.NewTimer(testTimeout).C:
		is.NoErr(ts.ctx.Err())
		is.NoErr(ctx.Err())
		is.Fail()
	case <-gotCallCh:
	}

	mt, b, err := conn.Read(ctx)
	is.NoErr(err)
	is.Equal(mt, websocket.MessageText)
	var m2 wsMsg
	m2.FillAlert(fooError)
	is.Equal(b, m2.Append(nil))
}

func TestReader_RespectsContextDone(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
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
	case <-time.NewTimer(time.Millisecond).C:
	case <-doneCh:
		is.Fail()
	}

	ts.cancel()

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}
}

func TestReader_RespectsJawsDone(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
	defer ts.Close()

	doneCh := make(chan struct{})
	inCh := make(chan wsMsg)
	client, server := Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	go func() {
		defer close(doneCh)
		wsReader(ts.ctx, nil, ts.jw.Done(), inCh, server)
	}()

	ts.jw.Close()
	msg := wsMsg{Jid: Jid(1234), What: what.Input}
	err := client.Write(ctx, websocket.MessageText, []byte(msg.Format()))
	is.NoErr(err)

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}
}

func TestWriter_SendsThePayload(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
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
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case outCh <- msg.Format():
	}

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}

	is.NoErr(err)
	is.Equal(mt, websocket.MessageText)
	is.Equal(string(b), msg.Format())

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-client.CloseRead(ts.ctx).Done():
	}
}

func TestWriter_RespectsContext(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
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
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
		return
	}
}

func TestWriter_RespectsJawsDone(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
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
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}
}

func TestWriter_RespectsOutboundClosed(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
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
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}

	is.NoErr(ts.rq.Context().Err())
}

func TestWriter_ReportsError(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
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
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case outCh <- msg.Format():
	}

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}

	err := context.Cause(ts.rq.Context())
	is.True(strings.Contains(err.Error(), "WebSocket closed"))
}

func TestReader_ReportsError(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
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
		t.FailNow()
	}

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}

	err = context.Cause(ts.rq.Context())
	is.True(strings.Contains(err.Error(), "WebSocket closed"))
}

func Test_wsParse_IncompleteFails(t *testing.T) {
	is := is.New(t)

	got, ok := wsParse(nil)
	is.True(!ok)
	is.Equal(got, wsMsg{})

	got, ok = wsParse([]byte("invalid\t\t\n")) // invalid What
	is.True(!ok)
	is.Equal(got, wsMsg{})

	got, ok = wsParse([]byte("Click\t\t")) // missing ending linefeed
	is.True(!ok)
	is.Equal(got, wsMsg{})

	got, ok = wsParse([]byte("Click\t\n")) // just one tab
	is.True(!ok)
	is.Equal(got, wsMsg{})

	got, ok = wsParse([]byte("\n\t\t\n")) // newline instead of What
	is.True(!ok)
	is.Equal(got, wsMsg{})

	got, ok = wsParse([]byte("Click\t\t\"\n\"\n")) // incorrectly quoted data
	is.True(!ok)
	is.Equal(got, wsMsg{})
}

func Test_wsParse_CompletePasses(t *testing.T) {
	tests := []struct {
		name string
		txt  string
		want wsMsg
	}{
		{"shortest", "Update\t\t\n", wsMsg{What: what.Update}},
		{"unquoted", "Input\tJid.1\ttrue\n", wsMsg{Jid: Jid(1), What: what.Input, Data: "true"}},
		{"normal", "Input\tJid.2\t\"c\"\n", wsMsg{Jid: Jid(2), What: what.Input, Data: "c"}},
		{"newline", "Input\tJid.3\t\"c\\nd\"\n", wsMsg{Jid: Jid(3), What: what.Input, Data: "c\nd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			got, ok := wsParse([]byte(tt.txt))
			if !ok {
				t.Log(got, tt.want)
			}
			is.True(ok)
			is.Equal(tt.want, got)
		})
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
