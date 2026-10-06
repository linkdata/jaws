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
			case <-writing:
			case <-ctx.Done():
				t.Fatal("writer outlived reload")
			}
		})
	}
}

func TestRequestReloadStopsQueuedEvents(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	started := make(chan struct{})
	var calls atomic.Int32
	item := &testUi{}
	rq.Register(item, func(*Element, string) error {
		if calls.Add(1) == 1 {
			close(started)
			<-rq.Context().Done()
		}
		return nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	for range 2 {
		select {
		case rq.InCh <- wire.WsMsg{Jid: jidForTag(rq.Request, item), What: what.Input}:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	rq.Reload()
	reloads := 0
	for {
		select {
		case msg, ok := <-rq.OutCh:
			if !ok {
				if calls.Load() != 1 || reloads != 1 {
					t.Fatalf("callbacks=%d reloads=%d, want 1 each", calls.Load(), reloads)
				}
				return
			}
			if reloads != 0 {
				t.Fatal("message followed final Reload")
			}
			if msg.What == what.Reload {
				reloads++
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
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
