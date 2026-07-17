package jaws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestRequestLateCancelDoesNotReachNextConnection(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()
	waitForServeLoop(t, jw)

	server := httptest.NewServer(jw)
	defer server.Close()
	newInitialRequest := func(path string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, server.URL+path, nil)
		r.RemoteAddr = "127.0.0.1:12345"
		return r
	}

	finished := jw.NewRequest(newInitialRequest("/first"))
	finishedKey := finished.JawsKey
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/jaws/"+finished.JawsKeyString(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{server.URL}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatal(err)
	}
	waitForRequestCount(t, jw, 0, time.Second)

	replacement := jw.NewRequest(newInitialRequest("/second"))
	defer jw.recycle(replacement)
	if replacement == finished {
		t.Fatal("NewRequest reused the finished Request identity")
	}
	if finished.JawsKey != finishedKey {
		t.Fatalf("finished Request key = %v, want stable key %v", finished.JawsKey, finishedKey)
	}
	replacementCtx := replacement.Context()
	finished.Cancel(errors.New("background operation completed after disconnect"))
	select {
	case <-replacementCtx.Done():
		t.Fatalf("late Cancel reached the next Request: %v", context.Cause(replacementCtx))
	default:
	}
}
