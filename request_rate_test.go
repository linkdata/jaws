package jaws

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/rate"
)

// broadcastFlood uses real WebSockets and a continuously reading one-Element
// subscriber. Senders do not subscribe to the shared tag. No receiver delays or
// artificial scheduling apply.
type broadcastFlood struct {
	ctx      context.Context
	victim   *Request
	senders  []*websocket.Conn
	received atomic.Int64
	changed  chan struct{}
	readDone chan struct{}
}

func newBroadcastFlood(tb testing.TB, maxRate int32, senders int) *broadcastFlood {
	tb.Helper()
	jw, err := New()
	if err != nil {
		tb.Fatal(err)
	}
	jw.MaxEventRate = maxRate
	ctx, cancel := context.WithTimeout(tb.Context(), 30*time.Second)
	go jw.Serve()
	srv := httptest.NewServer(jw)
	f := &broadcastFlood{ctx: ctx, changed: make(chan struct{}, 1), readDone: make(chan struct{})}
	var conns []*websocket.Conn
	tb.Cleanup(func() {
		cancel()
		for _, conn := range conns {
			_ = conn.CloseNow()
		}
		jw.Close()
		srv.Close()
	})
	const shared = tag.Tag("shared-log")
	const entry = template.HTML("<p>aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa</p>")
	connect := func(sender bool) (*Request, *websocket.Conn) {
		initial := httptest.NewRequest(http.MethodGet, srv.URL, nil).WithContext(ctx)
		initial.RemoteAddr = "127.0.0.1:12345"
		rw := httptest.NewRecorder()
		rq := jw.NewRequest(rw, initial)
		writer := testRequestWriter{rq: rq, Writer: rw}
		if sender {
			writer.Register(&testUi{}, func(*Element, string) error {
				jw.Append(shared, entry)
				return nil
			})
		} else {
			writer.Register(&testUi{}, shared)
		}
		connected := make(chan struct{})
		rq.SetConnectFn(func(*Request) error { close(connected); return nil })
		conn, _, dialErr := websocket.Dial(ctx, strings.Replace(srv.URL, "http", "ws", 1)+"/jaws/"+rq.JawsKeyString(), &websocket.DialOptions{
			HTTPHeader: http.Header{"Origin": {srv.URL}},
		})
		if dialErr != nil {
			tb.Fatal(dialErr)
		}
		conns = append(conns, conn)
		select {
		case <-connected:
		case <-ctx.Done():
			tb.Fatal(ctx.Err())
		}
		return rq, conn
	}
	var victimConn *websocket.Conn
	f.victim, victimConn = connect(false)
	go func() {
		defer close(f.readDone)
		for {
			_, data, readErr := victimConn.Read(ctx)
			if readErr != nil {
				return
			}
			for record := range bytes.Lines(data) {
				if msg, ok := wire.Parse(record); ok && msg.What == what.Append {
					f.received.Add(1)
				}
			}
			f.signal()
		}
	}()
	for range senders {
		_, conn := connect(true)
		conn.CloseRead(ctx)
		f.senders = append(f.senders, conn)
	}
	return f
}

func (f *broadcastFlood) signal() {
	select {
	case f.changed <- struct{}{}:
	default:
	}
}

func (f *broadcastFlood) wait(want int64) (err error) {
	for f.received.Load() < want {
		select {
		case <-f.changed:
		case <-f.readDone:
			return fmt.Errorf("victim disconnected after %d/%d appends: %w", f.received.Load(), want, context.Cause(f.victim.Context()))
		case <-f.ctx.Done():
			return f.ctx.Err()
		}
	}
	return
}

func TestWS_EventRateAndBroadcastBurst(t *testing.T) {
	for _, tt := range []struct {
		senders int
		rate    int32
	}{
		{senders: 1},
		{senders: 16},
		{senders: 1, rate: 25},
	} {
		t.Run(fmt.Sprintf("senders=%d/rate=%d", tt.senders, tt.rate), func(t *testing.T) {
			f := newBroadcastFlood(t, tt.rate, tt.senders)
			const records = 20
			msg := wire.WsMsg{Jid: 1, What: what.Input, Data: "send"}
			var payload []byte
			for range records {
				payload = msg.Append(payload)
			}
			start := time.Now()
			var writers sync.WaitGroup
			for _, conn := range f.senders {
				writers.Go(func() {
					if err := conn.Write(f.ctx, websocket.MessageText, payload); err != nil {
						t.Error(err)
					}
				})
			}
			writers.Wait()
			if err := f.wait(int64(tt.senders * records)); err != nil {
				t.Fatal(err)
			}
			maxRate := tt.rate
			if maxRate == 0 {
				maxRate = DefaultMaxEventRate
			}
			if elapsed := time.Since(start); elapsed < (records-1)*time.Second/time.Duration(maxRate) {
				t.Fatalf("event batch bypassed the rate limit: %v", elapsed)
			}
			if err := f.victim.Context().Err(); err != nil {
				t.Fatalf("victim cancelled: %v", context.Cause(f.victim.Context()))
			}
		})
	}
}

// BenchmarkWSBroadcastBurst measures synchronized senders against a continuously
// reading one-Element victim. Pacing is client-side so it isolates buffer capacity.
// Run with -cpu=1,4 to keep 16 senders in both cases and -benchtime below 30s.
// Compare delivery and disconnect metrics; ns/op includes client pacing.
func BenchmarkWSBroadcastBurst(b *testing.B) {
	for _, eventsPerSecond := range []int32{25, 400} {
		b.Run(fmt.Sprintf("rate=%d", eventsPerSecond), func(b *testing.B) {
			parallelism := max(1, 16/runtime.GOMAXPROCS(0))
			senders := parallelism * runtime.GOMAXPROCS(0)
			f := newBroadcastFlood(b, -1, senders)
			ctx, cancel := context.WithCancel(f.ctx)
			ticks := make([]chan struct{}, senders)
			for i := range ticks {
				ticks[i] = make(chan struct{}, 1)
			}
			clockDone := make(chan struct{})
			go func() {
				defer close(clockDone)
				limiter := rate.Limiter{CloseCh: ctx.Done()}
				for ctx.Err() == nil {
					limiter.Wait(&eventsPerSecond)
					for _, tick := range ticks {
						select {
						case tick <- struct{}{}:
						default:
						}
					}
				}
			}()
			var worker atomic.Int32
			event := wire.WsMsg{Jid: 1, What: what.Input, Data: "send"}
			msg := event.Append(nil)
			b.SetParallelism(parallelism)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := int(worker.Add(1)) - 1
				for pb.Next() {
					select {
					case <-ticks[i]:
					case <-ctx.Done():
						b.Error(ctx.Err())
						return
					}
					if err := f.senders[i].Write(ctx, websocket.MessageText, msg); err != nil {
						b.Error(err)
						return
					}
				}
			})
			cancel()
			<-clockDone
			err := f.wait(int64(b.N))
			b.StopTimer()
			if err != nil && f.victim.Context().Err() == nil {
				b.Fatal(err)
			}
			var disconnected float64
			if f.victim.Context().Err() != nil {
				disconnected = 1
			}
			b.ReportMetric(disconnected, "victim-disconnects")
			b.ReportMetric(100*float64(f.received.Load())/float64(b.N), "delivered-%")
		})
	}
}
