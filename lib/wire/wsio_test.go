package wire

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/what"
)

func TestReadLoop_RespectsContextDone(t *testing.T) {
	msg := WsMsg{Jid: jid.Jid(1234), What: what.Input}
	inCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	readDoneCh := make(chan struct{})
	go func() {
		defer close(readDoneCh)
		ReadLoop(ctx, nil, jawsDoneCh, inCh, time.Hour, time.Hour, server)
	}()

	writeCtx, writeCancel := context.WithTimeout(t.Context(), 3*time.Second)
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
	synctest.Test(t, func(t *testing.T) {
		msg := WsMsg{Jid: jid.Jid(1234), What: what.Input}
		inCh := make(chan WsMsg)
		doneCh := make(chan struct{})
		client, server := pipe(t)
		ctx, cancel := context.WithCancelCause(t.Context())
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		go func() {
			ReadLoop(ctx, cancel, doneCh, inCh, time.Hour, time.Hour, server)
			close(loopDone)
		}()

		if err := client.Write(ctx, websocket.MessageText, []byte(msg.Format())); err != nil {
			t.Fatal(err)
		}
		// ReadLoop is durably blocked sending the decoded message to inCh.
		synctest.Wait()
		close(doneCh)
		synctest.Wait()
		assertClosedNow(t, loopDone, "ReadLoop")
		if err := ctx.Err(); err != nil {
			t.Fatalf("parent context was canceled: %v", err)
		}
	})
}

func TestReadLoop_RespectsDoneWhileReading(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		inCh := make(chan WsMsg)
		client, server := pipe(t)
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		go func() {
			ReadLoop(ctx, cancel, doneCh, inCh, time.Hour, time.Hour, server)
			close(loopDone)
		}()

		// ReadLoop is durably blocked in ws.Read on the idle peer.
		synctest.Wait()
		close(doneCh)
		synctest.Wait()
		assertClosedNow(t, loopDone, "ReadLoop")
		if err := ctx.Err(); err != nil {
			t.Fatalf("parent context was canceled: %v", err)
		}
	})
}

func TestReadWriteLoop_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		want []WsMsg
	}{
		{
			name: "single record",
			want: []WsMsg{{Jid: 1, What: what.Input, Data: "value"}},
		},
		{
			name: "batched records",
			want: []WsMsg{
				{Jid: 1, What: what.Input, Data: "line one\nline two"},
				{Jid: 2, What: what.Set, Data: `state={"value":1}`},
				{Jid: 3, What: what.Call, Data: `notify=["done"]`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outCh := make(chan WsMsg, len(tt.want))
			for _, msg := range tt.want {
				outCh <- msg
			}
			close(outCh)

			inCh := make(chan WsMsg, len(tt.want))
			doneCh := make(chan struct{})
			client, server := pipe(t)
			defer func() { _ = client.CloseNow() }()
			defer func() { _ = server.CloseNow() }()

			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			readDoneCh := make(chan struct{})
			go func() {
				defer close(readDoneCh)
				ReadLoop(ctx, nil, doneCh, inCh, time.Hour, time.Hour, server)
			}()
			writeDoneCh := make(chan struct{})
			go func() {
				defer close(writeDoneCh)
				WriteLoop(ctx, nil, doneCh, outCh, time.Hour, client)
			}()

			var got []WsMsg
			for msg := range inCh {
				got = append(got, msg)
			}
			waitDone(t, readDoneCh, "ReadLoop after peer close")
			waitDone(t, writeDoneCh, "WriteLoop after outbound close")
			if !slices.Equal(got, tt.want) {
				t.Fatalf("round trip = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadLoop_SkipsMalformedRecords(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		want := []WsMsg{
			{Jid: 1, What: what.Input, Data: "first"},
			{Jid: 2, What: what.Set, Data: "state=second"},
		}
		payload := want[0].Append(nil)
		payload = append(payload, "malformed\n\n"...)
		payload = want[1].Append(payload)
		payload = append(payload, "Input\tJid.3\t\"unterminated\""...)

		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		inCh := make(chan WsMsg, len(want))
		client, server := pipe(t)
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		go func() {
			ReadLoop(ctx, cancel, doneCh, inCh, time.Hour, time.Hour, server)
			close(loopDone)
		}()
		if err := client.Write(ctx, websocket.MessageText, payload); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if got := len(inCh); got != len(want) {
			t.Fatalf("queued messages = %d, want %d", got, len(want))
		}

		close(doneCh)
		synctest.Wait()
		assertClosedNow(t, loopDone, "ReadLoop")
		var got []WsMsg
		for msg := range inCh {
			got = append(got, msg)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("messages = %+v, want %+v", got, want)
		}
		if err := ctx.Err(); err != nil {
			t.Fatalf("parent context was canceled: %v", err)
		}
	})
}

func TestReadLoop_BatchedDeliveryIsInterruptible(t *testing.T) {
	for _, stop := range []string{"context", "done"} {
		t.Run(stop, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const count = 128
				msg := WsMsg{Jid: 1, What: what.Input, Data: "value"}
				payload := make([]byte, 0, count*len(msg.Format()))
				for range count {
					payload = msg.Append(payload)
				}

				ctx, cancel := context.WithCancelCause(t.Context())
				doneCh := make(chan struct{})
				inCh := make(chan WsMsg, 1)
				client, server := pipe(t)
				loopDone := make(chan struct{})
				defer closeWireBubble(cancel, client, server)()

				go func() {
					ReadLoop(ctx, cancel, doneCh, inCh, time.Hour, time.Hour, server)
					close(loopDone)
				}()
				if err := client.Write(ctx, websocket.MessageText, payload); err != nil {
					t.Fatal(err)
				}
				// The first record is buffered and ReadLoop is blocked delivering the
				// second record from the same WebSocket message.
				synctest.Wait()
				if stop == "context" {
					cancel(nil)
				} else {
					close(doneCh)
				}
				synctest.Wait()
				assertClosedNow(t, loopDone, "ReadLoop")

				var got []WsMsg
				for msg := range inCh {
					got = append(got, msg)
				}
				if want := []WsMsg{msg}; !slices.Equal(got, want) {
					t.Fatalf("messages = %+v, want %+v", got, want)
				}
				if stop == "done" {
					if err := ctx.Err(); err != nil {
						t.Fatalf("parent context was canceled: %v", err)
					}
				}
			})
		})
	}
}

func TestReadLoop_DoesNotPingWhileDelivering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		inCh := make(chan WsMsg)
		var pingCount atomic.Int32
		client, server := pipeWithDialOptions(t, websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool {
				pingCount.Add(1)
				return true
			},
		})
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		client.CloseRead(ctx)
		go func() {
			ReadLoop(ctx, cancel, doneCh, inCh, time.Second, time.Second, server)
			close(loopDone)
		}()

		want := WsMsg{Jid: 1, What: what.Input, Data: "value"}
		if err := client.Write(ctx, websocket.MessageText, want.Append(nil)); err != nil {
			t.Fatal(err)
		}
		// The complete message is waiting for the application. This local delivery
		// delay is not peer inactivity, so advancing well beyond both ping durations
		// must not send a probe.
		synctest.Wait()
		time.Sleep(3 * time.Second)
		synctest.Wait()
		if got := pingCount.Load(); got != 0 {
			t.Fatalf("pings while delivering = %d, want 0", got)
		}

		if got := <-inCh; got != want {
			t.Fatalf("message = %+v, want %+v", got, want)
		}
		synctest.Wait()
		time.Sleep(3 * time.Second)
		synctest.Wait()
		if got := pingCount.Load(); got != 3 {
			t.Fatalf("successful idle pings = %d, want 3", got)
		}
		if err := ctx.Err(); err != nil {
			t.Fatalf("parent context was canceled: %v", err)
		}

		close(doneCh)
		synctest.Wait()
		assertClosedNow(t, loopDone, "ReadLoop")
	})
}

func TestReadLoop_ReadActivitySupersedesPingFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		inCh := make(chan WsMsg, 1)
		firstPingCh := make(chan struct{})
		var pingCount atomic.Int32
		client, server := pipeWithDialOptions(t, websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool {
				if pingCount.Add(1) == 1 {
					close(firstPingCh)
					return false
				}
				return true
			},
		})
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		client.CloseRead(ctx)
		go func() {
			ReadLoop(ctx, cancel, doneCh, inCh, time.Second, 10*time.Second, server)
			close(loopDone)
		}()

		<-firstPingCh
		want := WsMsg{Jid: 1, What: what.Input, Data: "active"}
		sentAt := time.Now()
		if err := client.Write(ctx, websocket.MessageText, want.Append(nil)); err != nil {
			t.Fatal(err)
		}
		if got := <-inCh; got != want {
			t.Fatalf("message = %+v, want %+v", got, want)
		}
		if delay := time.Since(sentAt); delay != 0 {
			t.Fatalf("delivery during ping was delayed by %v", delay)
		}
		// The first ping deliberately receives no pong. The completed data read
		// supersedes that probe, so its stale error must not cancel the connection.
		synctest.Wait()
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if err := ctx.Err(); err != nil {
			t.Fatalf("read activity did not supersede ping failure: %v", context.Cause(ctx))
		}

		close(doneCh)
		synctest.Wait()
		assertClosedNow(t, loopDone, "ReadLoop")
	})
}

func TestReadLoop_ReportsUnresponsivePeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		inCh := make(chan WsMsg)
		client, server := pipeWithDialOptions(t, websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool {
				return false // Read the ping but deliberately suppress the required pong.
			},
		})
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		client.CloseRead(ctx)
		go func() {
			ReadLoop(ctx, cancel, doneCh, inCh, time.Second, time.Second, server)
			close(loopDone)
		}()

		// The peer consumes the ping but violates the protocol by withholding its pong.
		time.Sleep(2 * time.Second)
		synctest.Wait()
		assertClosedNow(t, loopDone, "ReadLoop")
		if err := context.Cause(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("context cause = %T(%v), want deadline exceeded", err, err)
		}
	})
}

func TestWriteLoop_SendsThePayload(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, time.Hour, server)
	}()

	var mt websocket.MessageType
	var b []byte
	var err error
	readDoneCh := make(chan struct{})
	go func() {
		defer close(readDoneCh)
		readCtx, readCancel := context.WithTimeout(t.Context(), 3*time.Second)
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
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()

	msg := WsMsg{Jid: jid.Jid(1234)}
	outCh <- msg
	outCh <- msg
	close(outCh)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, time.Hour, server)
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
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()

	msg := WsMsg{Jid: jid.Jid(1234)}
	outCh <- msg
	close(outCh)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, time.Hour, server)
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

func TestWriteLoop_SplitsAtBatchLimit(t *testing.T) {
	// Drive the outbound backlog well past writeBatchLimit so writeData must split
	// it across more than one frame. The channel is buffered, pre-filled and closed,
	// so the coalescing loop is deterministic: it stops only on reaching the batch
	// limit or draining the (closed) channel, never on a transient empty read.
	msg := WsMsg{Jid: jid.Jid(1234), What: what.Inner, Data: strings.Repeat("x", 4096)}
	frame := msg.Format()
	count := (2*writeBatchLimit)/len(frame) + 2

	outCh := make(chan WsMsg, count)
	for i := 0; i < count; i++ {
		outCh <- msg
	}
	close(outCh)

	jawsDoneCh := make(chan struct{})
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()
	// Frames at the batch limit exceed coder/websocket's default 32 KB per-message
	// read limit; in production the browser (not a Go reader) consumes them, so
	// lift the limit here to read the server's large outbound frames.
	client.SetReadLimit(-1)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, time.Hour, server)
	}()

	var frames [][]byte
	var total []byte
	for {
		mt, b, err := client.Read(ctx)
		if err != nil {
			break
		}
		if mt != websocket.MessageText {
			t.Fatalf("frame type = %v, want MessageText", mt)
		}
		frames = append(frames, append([]byte(nil), b...))
		total = append(total, b...)
	}

	waitDone(t, writeDoneCh, "WriteLoop after backlog drained")

	if len(frames) < 2 {
		t.Fatalf("got %d frame(s), want the backlog split across more than one", len(frames))
	}
	// Nothing is dropped, duplicated or reordered: the concatenation of every frame
	// equals the concatenation of every queued message.
	if want := strings.Repeat(frame, count); string(total) != want {
		t.Fatalf("reassembled %d bytes across %d frames, want %d bytes", len(total), len(frames), len(want))
	}
	// Every frame is smaller than the soft limit plus one record, and every frame
	// except the last coalesces up to the limit before flushing.
	for i, f := range frames {
		if i < len(frames)-1 && len(f) < writeBatchLimit {
			t.Fatalf("frame %d is %d bytes, want >= writeBatchLimit (%d)", i, len(f), writeBatchLimit)
		}
		if len(f) >= writeBatchLimit+len(frame) {
			t.Fatalf("frame %d is %d bytes, want < %d", i, len(f), writeBatchLimit+len(frame))
		}
	}
}

func TestWriteLoop_RespectsContext(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()
	client.CloseRead(t.Context())

	ctx, cancel := context.WithCancel(t.Context())
	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, time.Hour, server)
	}()

	cancel()
	waitDone(t, writeDoneCh, "WriteLoop after context cancel")
}

func TestWriteLoop_RespectsDone(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()
	client.CloseRead(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, time.Hour, server)
	}()

	close(jawsDoneCh)
	waitDone(t, writeDoneCh, "WriteLoop after done close")
}

func TestWriteLoop_RespectsDoneWhileWriting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		outCh := make(chan WsMsg, 1)
		outCh <- WsMsg{
			Jid:  jid.Jid(1234),
			What: what.Inner,
			Data: strings.Repeat("x", writeBatchLimit),
		}
		client, server := pipe(t)
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		go func() {
			WriteLoop(ctx, cancel, doneCh, outCh, time.Hour, server)
			close(loopDone)
		}()

		// The large frame is durably blocked because the peer does not read.
		synctest.Wait()
		close(doneCh)
		synctest.Wait()
		assertClosedNow(t, loopDone, "WriteLoop")
		if err := ctx.Err(); err != nil {
			t.Fatalf("parent context was canceled: %v", err)
		}
	})
}

func TestWriteLoop_ReportsUnresponsivePeer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		outCh := make(chan WsMsg, 1)
		outCh <- WsMsg{
			Jid:  jid.Jid(1234),
			What: what.Inner,
			Data: strings.Repeat("x", writeBatchLimit),
		}
		client, server := pipe(t)
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		go func() {
			WriteLoop(ctx, cancel, doneCh, outCh, time.Second, server)
			close(loopDone)
		}()

		// The peer does not read, so the WebSocket write remains blocked until its
		// operation timeout closes the connection.
		time.Sleep(time.Second)
		synctest.Wait()
		assertClosedNow(t, loopDone, "WriteLoop")
		if err := context.Cause(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("context cause = %T(%v), want deadline exceeded", err, err)
		}
	})
}

func TestWriteLoop_RespectsOutboundClosed(t *testing.T) {
	outCh := make(chan WsMsg)
	jawsDoneCh := make(chan struct{})
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()
	client.CloseRead(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, nil, jawsDoneCh, outCh, time.Hour, server)
	}()

	close(outCh)
	waitDone(t, writeDoneCh, "WriteLoop after outbound close")
}

func TestWriteLoop_ReportsError(t *testing.T) {
	outCh := make(chan WsMsg, 1)
	jawsDoneCh := make(chan struct{})
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()
	client.CloseRead(t.Context())
	_ = server.CloseNow()

	ctx, cancel := context.WithCancelCause(t.Context())
	writeDoneCh := make(chan struct{})
	go func() {
		defer close(writeDoneCh)
		WriteLoop(ctx, cancel, jawsDoneCh, outCh, time.Hour, server)
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
	client, server := pipe(t)
	defer func() { _ = client.CloseNow() }()
	defer func() { _ = server.CloseNow() }()
	client.CloseRead(t.Context())
	_ = server.CloseNow()

	ctx, cancel := context.WithCancelCause(t.Context())
	readDoneCh := make(chan struct{})
	go func() {
		defer close(readDoneCh)
		ReadLoop(ctx, cancel, jawsDoneCh, inCh, time.Hour, time.Hour, server)
	}()

	waitDone(t, readDoneCh, "ReadLoop after read error")

	err := context.Cause(ctx)
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("%T(%v)", err, err)
	}
}

func TestReportError_IgnoresDone(t *testing.T) {
	doneCh := make(chan struct{})
	close(doneCh)
	reportError(t.Context(), doneCh, func(err error) {
		t.Fatalf("reported shutdown error: %v", err)
	}, errors.New("websocket closed"))
}

func TestReportError_IgnoresContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	reportError(ctx, make(chan struct{}), func(err error) {
		t.Fatalf("reported canceled-context error: %v", err)
	}, errors.New("websocket closed"))
}

func TestReadLoop_RespectsDoneWhileWaitingForPong(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		doneCh := make(chan struct{})
		inCh := make(chan WsMsg)
		pingSeen := make(chan struct{})
		client, server := pipeWithDialOptions(t, websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool {
				close(pingSeen)
				return false // Suppress the automatic pong.
			},
		})
		loopDone := make(chan struct{})
		defer closeWireBubble(cancel, client, server)()

		client.CloseRead(ctx)
		go func() {
			ReadLoop(ctx, cancel, doneCh, inCh, time.Second, time.Hour, server)
			close(loopDone)
		}()

		<-pingSeen
		// The idle watchdog is waiting for the deliberately omitted pong while its
		// socket reader remains in the concurrent read required by Ping.
		synctest.Wait()
		close(doneCh)
		synctest.Wait()
		assertClosedNow(t, loopDone, "ReadLoop")
		if err := ctx.Err(); err != nil {
			t.Fatalf("parent context was canceled: %v", err)
		}
	})
}

func waitDone(t *testing.T, doneCh <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for %s", what)
	}
}

func assertClosedNow(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	default:
		t.Fatalf("%s did not return", what)
	}
}

func closeWireBubble(cancel context.CancelCauseFunc, conns ...*websocket.Conn) func() {
	return func() {
		cancel(nil)
		for _, conn := range conns {
			_ = conn.CloseNow()
		}
		synctest.Wait()
	}
}

// adapted from nhooyr.io/websocket/internal/test/wstest.Pipe
func pipe(t *testing.T) (clientConn, serverConn *websocket.Conn) {
	t.Helper()
	return pipeWithDialOptions(t, websocket.DialOptions{})
}

func pipeWithDialOptions(t *testing.T, dialOpts websocket.DialOptions) (clientConn, serverConn *websocket.Conn) {
	t.Helper()
	dialOpts.HTTPClient = &http.Client{
		Transport: fakeTransport{
			h: func(w http.ResponseWriter, r *http.Request) {
				serverConn, _ = websocket.Accept(w, r, nil)
			},
		},
	}
	clientConn, _, _ = websocket.Dial(t.Context(), "ws://localhost", &dialOpts)
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
