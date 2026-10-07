package wire

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/what"
)

func TestReadLoopEventRate(t *testing.T) {
	for _, maxRate := range []int32{-1, 0, 100} {
		for _, batched := range []bool{false, true} {
			t.Run(fmt.Sprintf("rate=%d/batched=%t", maxRate, batched), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancelCause(t.Context())
					client, server := pipe(t)
					defer closeWireBubble(cancel, client, server)()
					inCh := make(chan WsMsg)
					go ReadLoop(ctx, cancel, nil, inCh, time.Hour, time.Hour, server, maxRate)
					want := []WsMsg{
						{Jid: 1, What: what.Remove, Data: "Jid.2"},
						{Jid: 1, What: what.Input, Data: "first"},
						{Jid: 1, What: what.Remove, Data: "Jid.3"},
						{Jid: 1, What: what.Click, Data: "1 1 0 click"},
						{Jid: 1, What: what.ContextMenu, Data: "1 1 0 menu"},
						{Jid: 1, What: what.JsVar, Data: "value=2"},
						{Jid: 1, What: what.Remove, Data: "Jid.4"},
					}
					go func() {
						var payload []byte
						for _, msg := range want {
							payload = msg.Append(payload)
							if !batched {
								if err := client.Write(ctx, websocket.MessageText, payload); err != nil {
									t.Error(err)
									return
								}
								payload = payload[:0]
							}
						}
						if batched {
							if err := client.Write(ctx, websocket.MessageText, payload); err != nil {
								t.Error(err)
							}
						}
					}()
					start := time.Now()
					var elapsed time.Duration
					for _, msg := range want {
						if maxRate > 0 && msg.What != what.Remove {
							elapsed += time.Second / time.Duration(maxRate)
						}
						if got := <-inCh; got != msg {
							t.Fatalf("record = %+v, want %+v", got, msg)
						}
						if got := time.Since(start); got != elapsed {
							t.Fatalf("%v arrived after %v, want %v", msg.What, got, elapsed)
						}
					}
					// Idle time permits one immediate event, not an accumulated burst.
					time.Sleep(time.Second)
					start = time.Now()
					msg := want[1]
					if err := client.Write(ctx, websocket.MessageText, msg.Append(msg.Append(nil))); err != nil {
						t.Fatal(err)
					}
					for i := range 2 {
						if got := <-inCh; got != msg {
							t.Fatalf("record after idle = %+v", got)
						}
						elapsed = 0
						if maxRate > 0 {
							elapsed = time.Duration(i) * time.Second / time.Duration(maxRate)
						}
						if got := time.Since(start); got != elapsed {
							t.Fatalf("record %d after idle arrived after %v, want %v", i, got, elapsed)
						}
					}
				})
			})
		}
	}
}

func TestReadLoopRateLimitShutdown(t *testing.T) {
	for _, maxRate := range []int32{10, 100} {
		for _, stop := range []string{"context", "done"} {
			t.Run(fmt.Sprintf("rate=%d/stop=%s", maxRate, stop), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancelCause(t.Context())
					client, server := pipe(t)
					defer closeWireBubble(cancel, client, server)()
					inCh := make(chan WsMsg, 2)
					done := make(chan struct{})
					loopDone := make(chan struct{})
					go func() {
						ReadLoop(ctx, cancel, done, inCh, time.Hour, time.Hour, server, maxRate)
						close(loopDone)
					}()
					msg := WsMsg{Jid: 1, What: what.Input}
					if err := client.Write(ctx, websocket.MessageText, msg.Append(msg.Append(nil))); err != nil {
						t.Fatal(err)
					}
					synctest.Wait()
					start := time.Now()
					if stop == "done" {
						close(done)
					} else {
						cancel(nil)
					}
					<-loopDone
					if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
						t.Fatalf("shutdown took %v", elapsed)
					}
					if msg, ok := <-inCh; ok {
						t.Fatalf("delivered record after shutdown: %+v", msg)
					}
					if stop == "done" && ctx.Err() != nil {
						t.Fatalf("shutdown reported a transport error: %v", context.Cause(ctx))
					}
				})
			})
		}
	}
}

func TestReadLoopDoesNotPingWhileRateLimited(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(t.Context())
		var pings atomic.Int32
		client, server := pipeWithDialOptions(t, websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool { pings.Add(1); return true },
		})
		defer closeWireBubble(cancel, client, server)()
		client.CloseRead(ctx)
		inCh := make(chan WsMsg)
		go ReadLoop(ctx, cancel, nil, inCh, time.Millisecond, time.Second, server, 10)
		msg := WsMsg{Jid: 1, What: what.Input}
		if err := client.Write(ctx, websocket.MessageText, msg.Append(msg.Append(nil))); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if got := <-inCh; got != msg {
				t.Fatalf("record = %+v", got)
			}
		}
		if got := pings.Load(); got != 0 {
			t.Fatalf("pings during pacing = %d", got)
		}
	})
}
