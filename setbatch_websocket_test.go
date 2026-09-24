package jaws

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func TestSetBatchDistinctPathsKeepReadingWebSocketConnected(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	stateTag := tag.Tag("state")
	updated := make(chan struct{}, 1)
	elem := ts.rq.NewElement(&testUi{updateFn: func(*Element) {
		updated <- struct{}{}
	}})
	elem.Tag(stateTag)

	conn, response, err := ts.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if response.StatusCode != 101 {
		t.Fatalf("WebSocket status = %d, want 101", response.StatusCode)
	}
	<-ts.connectedCh

	readCtx, cancelRead := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelRead()
	messages := make(chan wire.WsMsg, 32)
	readErrors := make(chan error, 1)
	go func() {
		for {
			messageType, data, readErr := conn.Read(readCtx)
			if readErr != nil {
				readErrors <- readErr
				return
			}
			if messageType != websocket.MessageText {
				readErrors <- fmt.Errorf("WebSocket message type = %v, want text", messageType)
				return
			}
			for record := range bytes.Lines(data) {
				msg, ok := wire.Parse(record)
				if !ok {
					readErrors <- fmt.Errorf("invalid WebSocket record %q", record)
					return
				}
				messages <- msg
			}
		}
	}()

	// Start just after an update pass so the burst fits within one tick.
	ts.jw.Dirty(stateTag)
	select {
	case <-updated:
	case <-readCtx.Done():
		t.Fatal("timed out waiting for update tick")
	}

	for i := range 20 {
		ts.jw.Broadcast(wire.Message{
			Dest: stateTag,
			What: what.Set,
			Data: fmt.Sprintf("f%d=%d", i, i),
		})
		if i < 19 {
			time.Sleep(4 * time.Millisecond)
		}
	}
	ts.jw.Broadcast(wire.Message{What: what.Alert, Data: "barrier"})

	for i := range 21 {
		select {
		case msg := <-messages:
			if i < 20 {
				if msg.What != what.Set || msg.Jid != elem.Jid() || msg.Data != fmt.Sprintf("f%d=%d", i, i) {
					t.Fatalf("message %d = %#v, want ordered path Set", i, msg)
				}
			} else if msg.What != what.Alert || msg.Data != "barrier" {
				t.Fatalf("last message = %#v, want barrier Alert", msg)
			}
		case err := <-readErrors:
			t.Fatalf("WebSocket read failed after %d messages: %v; request cause: %v", i, err, context.Cause(ts.rq.Context()))
		case <-ts.rq.Context().Done():
			t.Fatalf("Request cancelled after %d messages: %v", i, context.Cause(ts.rq.Context()))
		case <-readCtx.Done():
			t.Fatalf("timed out after %d messages: %v", i, context.Cause(ts.rq.Context()))
		}
	}
	if cause := context.Cause(ts.rq.Context()); cause != nil {
		t.Fatalf("Request cancelled after barrier: %v", cause)
	}
}
