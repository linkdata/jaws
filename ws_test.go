package jaws

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
	"nhooyr.io/websocket"
)

type testServer struct {
	is     *is.I
	jw     *Jaws
	ctx    context.Context
	cancel context.CancelFunc
	rq     *Request
	srv    *httptest.Server
}

func newTestServer(is *is.I) *testServer {
	jw := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	rq := jw.NewRequest(ctx, "")
	srv := httptest.NewServer(rq)
	return &testServer{
		is:     is,
		jw:     jw,
		ctx:    ctx,
		cancel: cancel,
		rq:     rq,
		srv:    srv,
	}
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
	rq := jw.NewRequest(context.Background(), "")

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

	ts.rq.RegisterEventFn("foo", func(rq *Request, id, evt, val string) error {
		close(gotCallCh)
		return fooError
	})

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusSwitchingProtocols)
	defer conn.Close(websocket.StatusNormalClosure, "")

	msg := &Message{Elem: "foo"}
	ctx, cancel := context.WithTimeout(ts.ctx, time.Second*3)
	defer cancel()

	err = conn.Write(ctx, websocket.MessageText, []byte(msg.Format()))
	is.NoErr(err)
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCallCh:
	}

	mt, b, err := conn.Read(ctx)
	is.NoErr(err)
	is.Equal(mt, websocket.MessageText)
	is.Equal(string(b), makeAlertDangerMessage(fooError).Format())
}

func TestReader_RespectsContextDone(t *testing.T) {
	is := is.New(t)
	ts := newTestServer(is)
	defer ts.Close()

	msg := &Message{Elem: "foo"}
	doneCh := make(chan struct{})
	inCh := make(chan *Message)
	client, server := Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	go func() {
		defer close(doneCh)
		wsReader(ts.ctx, ts.jw.Done(), inCh, server)
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
	inCh := make(chan *Message)
	client, server := Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	go func() {
		defer close(doneCh)
		wsReader(ts.ctx, ts.jw.Done(), inCh, server)
	}()

	ts.jw.Close()
	msg := &Message{Elem: "foo"}
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

	outCh := make(chan *Message)
	defer close(outCh)
	client, server := Pipe()
	msg := &Message{Elem: "foo"}

	go wsWriter(ts.ctx, ts.jw.Done(), outCh, server)

	var mt websocket.MessageType
	var b []byte
	var err error
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		mt, b, err = client.Read(ts.ctx)
		ts.cancel()
	}()

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case outCh <- msg:
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
	outCh := make(chan *Message)
	defer close(outCh)
	client, server := Pipe()
	client.CloseRead(context.Background())

	go func() {
		defer close(doneCh)
		wsWriter(ts.ctx, ts.jw.Done(), outCh, server)
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
	outCh := make(chan *Message)
	defer close(outCh)
	client, server := Pipe()
	client.CloseRead(ts.ctx)

	go func() {
		defer close(doneCh)
		wsWriter(ts.ctx, ts.jw.Done(), outCh, server)
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
	outCh := make(chan *Message)
	client, server := Pipe()
	client.CloseRead(ts.ctx)

	go func() {
		defer close(doneCh)
		wsWriter(ts.ctx, ts.jw.Done(), outCh, server)
	}()

	close(outCh)

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-doneCh:
	}
}

func Test_wsParse_IncompleteFails(t *testing.T) {
	is := is.New(t)

	is.Equal(wsParse(nil), nil)            // duh
	is.Equal(wsParse([]byte("\n")), nil)   // missing Elem
	is.Equal(wsParse([]byte("id\n")), nil) // missing What
}

func Test_wsParse_CompletePasses(t *testing.T) {
	tests := []struct {
		name string
		txt  string
		want *Message
	}{
		{"shortest", " \n\n", &Message{Elem: " "}},
		{"normal", "a\nb\nc", &Message{Elem: "a", What: "b", Data: "c"}},
		{"newline", "a\nb\nc\nd", &Message{Elem: "a", What: "b", Data: "c\nd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := is.New(t)
			if got := wsParse([]byte(tt.txt)); !reflect.DeepEqual(got, tt.want) {
				is.Equal(tt.want, got)
			}
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
