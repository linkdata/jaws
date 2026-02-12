package core

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

func TestCoverage_WsMsgAppendNegativeJidAndServeWithTimeoutBounds(t *testing.T) {
	msg := WsMsg{Jid: jid.Jid(-1), What: what.Update, Data: "raw\tdata"}
	if got := string(msg.Append(nil)); got != "Update\traw\tdata\n" {
		t.Fatalf("unexpected ws append result %q", got)
	}
	msg = WsMsg{Jid: 1, What: what.Call, Data: `fn={"a":1}`}
	if got := string(msg.Append(nil)); !strings.Contains(got, `fn={"a":1}`) || strings.Contains(got, `"fn={"`) {
		t.Fatalf("unexpected ws append quoted call payload %q", got)
	}

	// Min interval clamp path.
	jwMin, err := New()
	if err != nil {
		t.Fatal(err)
	}
	doneMin := make(chan struct{})
	go func() {
		jwMin.ServeWithTimeout(time.Nanosecond)
		close(doneMin)
	}()
	jwMin.Close()
	select {
	case <-doneMin:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout(min)")
	}

	// Max interval clamp path.
	jwMax, err := New()
	if err != nil {
		t.Fatal(err)
	}
	doneMax := make(chan struct{})
	go func() {
		jwMax.ServeWithTimeout(10 * time.Second)
		close(doneMax)
	}()
	jwMax.Close()
	select {
	case <-doneMax:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout(max)")
	}
}

func TestCoverage_ServeWithTimeoutFullSubscriberChannel(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRequest("GET", "/", nil))
	msgCh := make(chan Message) // unbuffered: always full when nobody receives
	done := make(chan struct{})
	go func() {
		jw.ServeWithTimeout(50 * time.Millisecond)
		close(done)
	}()
	jw.subCh <- subscription{msgCh: msgCh, rq: rq}
	// Ensure ServeWithTimeout has consumed the subscription before broadcast.
	for i := 0; i <= cap(jw.subCh); i++ {
		jw.subCh <- subscription{}
	}
	jw.bcastCh <- Message{What: what.Alert, Data: "x"}

	waitUntil := time.Now().Add(time.Second)
	closed := false
	for !closed && time.Now().Before(waitUntil) {
		select {
		case _, ok := <-msgCh:
			closed = !ok
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if !closed {
		t.Fatal("expected subscriber channel to be closed when full")
	}

	jw.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ServeWithTimeout exit")
	}
}
