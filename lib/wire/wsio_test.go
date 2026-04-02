package wire

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/what"
)

func TestReadLoop_RespectsContextDone(t *testing.T) {
	msg := WsMsg{Jid: jid.Jid(1234), What: what.Input}
	inCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	readDoneCh := make(chan struct{})
	go func() {
		defer close(readDoneCh)
		ReadLoop(ctx, nil, jawsDoneCh, inCh, server)
	}()

	writeCtx, writeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer writeCancel()
	if err := client.Write(writeCtx, websocket.MessageText, []byte(msg.Format())); err != nil {
		t.Fatal(err)
	}

	// ReadLoop should now be blocked trying to send the decoded message.
	select {
	case <-readDoneCh:
		t.Fatal("did not block")
	case <-time.After(time.Millisecond):
	}

	cancel()
	waitDone(t, readDoneCh, "ReadLoop after context cancel")
}

func TestReadLoop_RespectsDone(t *testing.T) {
	msg := WsMsg{Jid: jid.Jid(1234), What: what.Input}
	inCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	readDoneCh := make(chan struct{})
	go func() {
		defer close(readDoneCh)
		ReadLoop(ctx, nil, jawsDoneCh, inCh, server)
	}()

	if err := client.Write(ctx, websocket.MessageText, []byte(msg.Format())); err != nil {
		t.Fatal(err)
	}
	close(jawsDoneCh)
	waitDone(t, readDoneCh, "ReadLoop after done close")
}

func TestWriteLoop_SendsThePayload(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, server)
	}()

	var mt websocket.MessageType
	var b []byte
	var err error
	readDoneCh := make(chan struct{})
	go func() {
		defer close(readDoneCh)
		readCtx, readCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer readCancel()
		mt, b, err = client.Read(readCtx)
	}()

	msg := WsMsg{Jid: jid.Jid(1234)}
	select {
	case outCh <- msg:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout sending outbound message")
	}

	waitDone(t, readDoneCh, "websocket read")
	if err != nil {
		t.Fatal(err)
	}
	if mt != websocket.MessageText {
		t.Fatal(mt)
	}
	if string(b) != msg.Format() {
		t.Fatal(string(b))
	}

	cancel()
	_ = client.CloseNow()
	waitDone(t, writeDoneCh, "WriteLoop after context cancel")
}

func TestWriteLoop_ConcatenatesMessages(t *testing.T) {
	outCh := make(chan WsMsg, 2)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	msg := WsMsg{Jid: jid.Jid(1234)}
	outCh <- msg
	outCh <- msg
	close(outCh)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, server)
	}()

	mt, b, err := client.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mt != websocket.MessageText {
		t.Fatal(mt)
	}
	want := msg.Format() + msg.Format()
	if string(b) != want {
		t.Fatalf("got %q, want %q", string(b), want)
	}
	_ = client.CloseNow()
	waitDone(t, writeDoneCh, "WriteLoop after outbound close")
}

func TestWriteLoop_ConcatenatesMessagesClosedChannel(t *testing.T) {
	outCh := make(chan WsMsg, 2)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	msg := WsMsg{Jid: jid.Jid(1234)}
	outCh <- msg
	close(outCh)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, server)
	}()

	mt, b, err := client.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mt != websocket.MessageText {
		t.Fatal(mt)
	}
	want := msg.Format()
	if string(b) != want {
		t.Fatalf("got %q, want %q", string(b), want)
	}
	_ = client.CloseNow()
	waitDone(t, writeDoneCh, "WriteLoop after closed outbound")
}

func TestWriteLoop_RespectsContext(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()
	client.CloseRead(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, server)
	}()

	cancel()
	waitDone(t, writeDoneCh, "WriteLoop after context cancel")
}

func TestWriteLoop_RespectsDone(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()
	client.CloseRead(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, server)
	}()

	close(jawsDoneCh)
	waitDone(t, writeDoneCh, "WriteLoop after done close")
}

func TestWriteLoop_RespectsOutboundClosed(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()
	client.CloseRead(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, server)
	}()

	close(outCh)
	waitDone(t, writeDoneCh, "WriteLoop after outbound close")
}

func TestWriteLoop_ReportsError(t *testing.T) {
	outCh := make(chan WsMsg, 1)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()
	client.CloseRead(context.Background())
	server.CloseNow()

	ctx, cancel := context.WithCancelCause(context.Background())
	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, cancel, jawsDoneCh, outCh, server)
	}()

	outCh <- WsMsg{Jid: jid.Jid(1234)}
	waitDone(t, writeDoneCh, "WriteLoop after write error")

	err := context.Cause(ctx)
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("%T(%v)", err, err)
	}
}

func TestReadLoop_ReportsError(t *testing.T) {
	inCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()
	client.CloseRead(context.Background())
	server.CloseNow()

	ctx, cancel := context.WithCancelCause(context.Background())
	readDoneCh := make(chan struct{})
	go func() {
		defer close(readDoneCh)
		ReadLoop(ctx, cancel, jawsDoneCh, inCh, server)
	}()

	waitDone(t, readDoneCh, "ReadLoop after read error")

	err := context.Cause(ctx)
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("%T(%v)", err, err)
	}
}

func TestPingLoop_RespectsContextDone(t *testing.T) {
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pingDoneCh := make(chan struct{})
	go func() {
		defer close(pingDoneCh)
		PingLoop(ctx, nil, jawsDoneCh, time.Millisecond*10, time.Millisecond*10, server)
	}()

	cancel()
	waitDone(t, pingDoneCh, "PingLoop after context cancel")
}

func TestPingLoop_RespectsDone(t *testing.T) {
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pingDoneCh := make(chan struct{})
	go func() {
		defer close(pingDoneCh)
		PingLoop(ctx, nil, jawsDoneCh, time.Millisecond, time.Millisecond, server)
	}()

	close(jawsDoneCh)
	waitDone(t, pingDoneCh, "PingLoop after done close")
}

func TestPingLoop_ReportsErrorWhenPeerDoesNotPong(t *testing.T) {
	jawsDoneCh := make(chan struct{})
	client, server := pipe()
	defer client.CloseNow()
	defer server.CloseNow()

	ctx, cancel := context.WithCancelCause(context.Background())

	pingDoneCh := make(chan struct{})
	go func() {
		defer close(pingDoneCh)
		PingLoop(ctx, cancel, jawsDoneCh, 20*time.Millisecond, 20*time.Millisecond, server)
	}()

	waitDone(t, pingDoneCh, "PingLoop after ping timeout")

	err := context.Cause(ctx)
	if err == nil {
		t.Fatal("expected context cause from ping failure")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%T(%v)", err, err)
	}
}

func waitDone(t *testing.T, doneCh <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for %s", what)
	}
}

// adapted from nhooyr.io/websocket/internal/test/wstest.Pipe
func pipe() (clientConn, serverConn *websocket.Conn) {
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
