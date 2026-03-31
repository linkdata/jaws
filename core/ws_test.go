package jaws

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/core/assets"
	"github.com/linkdata/jaws/core/wire"
	"github.com/linkdata/jaws/what"
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
	jw, _ := New()
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
	ts.setInitialRequestOrigin()
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
		jawsKey := assets.JawsKeyValue(strings.TrimPrefix(r.URL.Path, "/jaws/"))
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

func (ts *testServer) setInitialRequestOrigin() {
	if ts.hr == nil {
		return
	}
	u, err := url.Parse(ts.srv.URL)
	if err != nil {
		return
	}
	ts.hr.Host = u.Host
	if ts.hr.URL != nil {
		ts.hr.URL.Host = u.Host
		ts.hr.URL.Scheme = u.Scheme
	}
}

func (ts *testServer) origin() string {
	scheme := "http"
	if ts.hr != nil && ts.hr.URL != nil && ts.hr.URL.Scheme != "" {
		scheme = ts.hr.URL.Scheme
	}
	host := ""
	if ts.hr != nil {
		host = ts.hr.Host
	}
	if host == "" {
		if u, err := url.Parse(ts.srv.URL); err == nil {
			host = u.Host
			if scheme == "" {
				scheme = u.Scheme
			}
		}
	}
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + host
}

func (ts *testServer) Dial() (*websocket.Conn, *http.Response, error) {
	hdr := http.Header{}
	hdr.Set("Origin", ts.origin())
	opts := &websocket.DialOptions{HTTPHeader: hdr}
	return websocket.Dial(ts.ctx, ts.Url(), opts)
}

func (ts *testServer) Close() {
	ts.cancel()
	ts.srv.Close()
	ts.jw.Close()
}

func TestWS_UpgradeRequired(t *testing.T) {
	jw, _ := New()
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

func TestWS_RejectsMissingOrigin(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), nil)
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("expected handshake to be rejected")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestWS_RejectsCrossOrigin(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	hdr := http.Header{}
	hdr.Set("Origin", "https://evil.invalid")
	conn, resp, err := websocket.Dial(ts.ctx, ts.Url(), &websocket.DialOptions{HTTPHeader: hdr})
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("expected handshake to be rejected")
	}
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestWS_ConnectFnFails(t *testing.T) {
	const nope = "nope"
	ts := newTestServer()
	defer ts.Close()
	ts.rq.SetConnectFn(func(_ *Request) error { return errors.New(nope) })

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if conn != nil {
		defer conn.Close(websocket.StatusNormalClosure, "")
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
	fooItem := &testUi{}
	testRequestWriter{rq: ts.rq, Writer: httptest.NewRecorder()}.Register(fooItem, func(e *Element, evt what.What, val string) error {
		close(gotCallCh)
		return fooError
	})

	conn, resp, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error(resp.StatusCode)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	msg := wire.WsMsg{Jid: jidForTag(ts.rq, fooItem), What: what.Input}
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
	var m2 wire.WsMsg
	m2.FillAlert(fooError)
	if !bytes.Equal(b, m2.Append(nil)) {
		t.Error(b)
	}
}
