package jaws

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
				if when == "pending" {
					t.Error("reload ran ConnectFn")
				}
				if when == "connecting" {
					rq.Reload()
					return rq.Context().Err()
				}
				close(connected)
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
			if when == "connected" {
				select {
				case <-connected:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
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

func TestRequestReloadStopsNewEvents(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	queued := make(chan struct{})
	var calls atomic.Int32
	item := &testUi{}
	rq.Register(item, func(*Element, string) error {
		switch calls.Add(1) {
		case 1:
			close(started)
			<-release
		case 2:
			close(queued)
		}
		return nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	send := func(msg wire.WsMsg) {
		t.Helper()
		select {
		case rq.InCh <- msg:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	msg := wire.WsMsg{Jid: jidForTag(rq.Request, item), What: what.Input}
	send(msg)
	send(msg)
	// The unbuffered handoff of a third message fences admission of both events.
	send(wire.WsMsg{})
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	rq.Reload()
	send(msg)
	send(wire.WsMsg{})
	select {
	case release <- struct{}{}:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-queued:
	case <-ctx.Done():
		t.Fatal("event queued before Reload did not run")
	}
	select {
	case msg := <-rq.OutCh:
		if msg.What != what.Reload {
			t.Fatalf("got %v, want Reload", msg)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if rq.Context().Err() != nil {
		t.Fatal("request cancelled before the writer disconnected")
	}
	rq.Cancel(nil) // Simulate the writer closing the connection after Reload.
	select {
	case <-rq.DoneCh:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if calls.Load() != 2 {
		t.Fatalf("callbacks=%d, want the two events accepted before Reload", calls.Load())
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
	if rq.reloading() {
		t.Fatal("Reload changed an already cancelled request")
	}
}

func TestRequestReloadStateTransitions(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	defer jw.Close()
	for _, when := range []string{"pending", "claimed", "running", "concurrent"} {
		t.Run(when, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			rq := jw.NewRequest(httptest.NewRecorder(), r)
			if when == "pending" {
				rq.Reload()
			}
			if jw.UseRequest(rq.JawsKey, r) != rq {
				t.Fatal("claim failed")
			}
			if when == "claimed" {
				rq.Reload()
			}
			reloaded := make(chan struct{})
			if when == "concurrent" {
				go func() {
					rq.Reload()
					close(reloaded)
				}()
			}
			if !rq.startServe() {
				t.Fatal("startServe failed")
			}
			if when == "concurrent" {
				<-reloaded
			}
			if when == "running" {
				rq.Reload()
			}
			if rq.loadState() != reqRunning || !rq.reloading() {
				t.Fatal("running request lost its reload flag")
			}
			if rq.startServe() || jw.UseRequest(rq.JawsKey, r) != nil {
				t.Fatal("reload allowed a duplicate connection")
			}
			rq.stopServe()
			if rq.loadState() != reqFinished || !rq.reloading() {
				t.Fatal("finished request lost its reload flag")
			}
		})
	}
}
