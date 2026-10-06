package jaws

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestRequestReload(t *testing.T) {
	for _, when := range []string{"pending", "connecting", "connected"} {
		t.Run(when, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			go jw.Serve()
			defer jw.Close()
			hs := httptest.NewServer(jw)
			defer hs.Close()
			hr := httptest.NewRequest(http.MethodGet, hs.URL+"/", nil)
			hr.RemoteAddr = "127.0.0.1:1"
			sess := jw.NewSession(httptest.NewRecorder(), hr)
			sess.Set("kept", true)
			rq := jw.NewRequest(httptest.NewRecorder(), hr)
			requestCtx := rq.Context()
			connected := make(chan struct{})
			rq.SetConnectFn(func(rq *Request) error {
				close(connected)
				if when == "connecting" {
					rq.Reload()
				}
				return nil
			})
			if when == "pending" {
				rq.Reload()
				rq.Reload()
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(hs.URL, "http")+"/jaws/"+rq.JawsKeyString(),
				&websocket.DialOptions{HTTPHeader: http.Header{"Origin": {hs.URL}}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.CloseNow() }()
			select {
			case <-connected:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if when == "connected" {
				rq.Reload()
				rq.Reload()
			}
			writing := make(chan struct{})
			go func() {
				defer close(writing)
				for conn.Write(ctx, websocket.MessageText, []byte("Input\tJid.1\tignored\n")) == nil {
				}
			}()
			reloads := 0
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
						t.Fatal(err)
					}
					break
				}
				for record := range bytes.Lines(data) {
					if msg, ok := wire.Parse(record); ok && msg.What == what.Reload {
						reloads++
					}
				}
			}
			if reloads != 1 || sess.Get("kept") != true || sess.Cookie().MaxAge < 0 {
				t.Fatalf("reloads=%d, session data=%v, cookie=%v", reloads, sess.Get("kept"), sess.Cookie())
			}
			select {
			case <-requestCtx.Done():
			case <-ctx.Done():
				t.Fatal("request outlived reload")
			}
			select {
			case <-writing:
			case <-ctx.Done():
				t.Fatal("writer outlived reload")
			}
		})
	}
}

func TestRequestReloadAllowsEventsUntilDisconnect(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	called := make(chan struct{})
	item := &testUi{}
	rq.Register(item, func(*Element, string) error {
		close(called)
		return nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	rq.Reload()
	select {
	case msg := <-rq.OutCh:
		if msg.What != what.Reload {
			t.Fatalf("got %v, want Reload", msg)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// The harness has no socket writer, so the request remains connected.
	select {
	case rq.InCh <- wire.WsMsg{Jid: jidForTag(rq.Request, item), What: what.Input}:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-called:
	case <-ctx.Done():
		t.Fatal("Reload stopped event dispatch before disconnection")
	}
}

func TestRequestReloadAfterCancel(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	rq.Cancel(nil)
	rq.Reload()
	if msgs := rq.getSendMsgs(); len(msgs) != 0 {
		t.Fatalf("Reload queued messages on a cancelled request: %v", msgs)
	}
}
