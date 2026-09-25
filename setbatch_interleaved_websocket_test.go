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

func TestSetBatchInterleavedDestinationsKeepReadingWebSocketConnected(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	a, b := tag.Tag("A"), tag.Tag("B")
	updated := make(chan struct{}, 1)
	aElem := ts.rq.NewElement(&testUi{updateFn: func(*Element) {
		updated <- struct{}{}
	}})
	aElem.Tag(a)
	bElem := ts.rq.NewElement(&testUi{})
	bElem.Tag(b)

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
	messages := make(chan wire.WsMsg, 64)
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
	ts.jw.Dirty(a)
	select {
	case <-updated:
	case <-readCtx.Done():
		t.Fatal("timed out waiting for update tick")
	}

	for i := range 16 {
		destinations := [2]tag.Tag{a, b}
		if i == 15 {
			// End with B then A so sorting the whole group by Jid would
			// deliver a stale final value for a shared browser name.
			destinations = [2]tag.Tag{b, a}
		}
		for _, dest := range destinations {
			value := i
			if dest == b {
				value += 100
			}
			ts.jw.Broadcast(wire.Message{
				Dest: dest,
				What: what.Set,
				Data: fmt.Sprintf("f%d=%d", i, value),
			})
			time.Sleep(3 * time.Millisecond)
		}
	}
	ts.jw.Broadcast(wire.Message{What: what.Alert, Data: "barrier"})

	for i := range 33 {
		select {
		case msg := <-messages:
			if i < 32 {
				isB := i%2 == 1
				if i/2 == 15 {
					isB = !isB
				}
				jid := aElem.Jid()
				value := i / 2
				if isB {
					jid = bElem.Jid()
					value += 100
				}
				wantData := fmt.Sprintf("f%d=%d", i/2, value)
				if msg.What != what.Set || msg.Jid != jid || msg.Data != wantData {
					t.Fatalf("message %d = %#v, want ordered Set for Jid %v with data %q", i, msg, jid, wantData)
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
