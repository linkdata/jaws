package jaws

import (
	"context"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// benchUI is a minimal comparable UI for populating a Request with Elements.
// It is self-contained so the benchmarks compile unchanged against released tags.
type benchUI struct{ n int }

func (benchUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (benchUI) JawsUpdate(*Element)                         {}

var benchRequestSink *Request

// newBenchRequest returns a Request seeded with n Elements (Jids 1..n, ascending).
func newBenchRequest(b *testing.B, n int) *Request {
	b.Helper()
	jw, err := New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { jw.Close() })
	rq := &Request{Jaws: jw}
	for i := 0; i < n; i++ {
		rq.NewElement(benchUI{n: i})
	}
	return rq
}

func newUnpooledBenchRequest(jw *Jaws) (rq *Request) {
	remoteIP := jw.clientIP(nil)
	jw.mu.Lock()
	defer jw.mu.Unlock()
	jw.limitPendingRequestsLocked(remoteIP)
	for rq == nil {
		jawsKey := jw.nonZeroRandomLocked()
		if _, ok := jw.requests[jawsKey]; !ok {
			rq = &Request{
				Jaws:   jw,
				tagMap: make(map[any][]*Element),
			}
			rq.mu.Lock()
			rq.JawsKey = jawsKey
			rq.lastWriteNano.Store(jw.nowNano())
			rq.remoteIP = remoteIP
			rq.ctx, rq.cancelFn = context.WithCancelCause(jw.BaseContext)
			rq.mu.Unlock()
			jw.requests[jawsKey] = rq
			jw.pending[rq.remoteIP] = append(jw.pending[rq.remoteIP], rq)
		}
	}
	return
}

func recycleUnpooledBenchRequest(jw *Jaws, rq *Request) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if rq.JawsKey != 0 {
		jw.removePendingRequestLocked(rq)
		delete(jw.requests, rq.JawsKey)
		rq.clearLocked()
	}
}

func populateBenchRequest(rq *Request, tags []any) {
	for i, tagValue := range tags {
		rq.Tag(rq.NewElement(benchUI{n: i}), tagValue)
	}
}

// BenchmarkRequestLifecyclePooling compares the current pooled Request lifecycle
// with a benchmark-only fresh-allocation lifecycle. It isolates the create,
// optional render-state population and recycle path; it does not measure HTTP or
// WebSocket serving.
func BenchmarkRequestLifecyclePooling(b *testing.B) {
	for _, c := range []struct {
		name  string
		elems int
	}{
		{"empty", 0},
		{"elems=100", 100},
		{"elems=1000", 1000},
	} {
		tags := make([]any, c.elems)
		for i := range tags {
			tags[i] = tag.Tag("bench-" + strconv.Itoa(i))
		}
		b.Run(c.name, func(b *testing.B) {
			b.Run("pooled", func(b *testing.B) {
				jw, err := New()
				if err != nil {
					b.Fatal(err)
				}
				b.Cleanup(func() { jw.Close() })

				rq := jw.NewRequest(nil)
				populateBenchRequest(rq, tags)
				jw.recycle(rq)

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rq = jw.NewRequest(nil)
					populateBenchRequest(rq, tags)
					jw.recycle(rq)
				}
				benchRequestSink = rq
			})
			b.Run("unpooled", func(b *testing.B) {
				jw, err := New()
				if err != nil {
					b.Fatal(err)
				}
				b.Cleanup(func() { jw.Close() })

				var rq *Request
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rq = newUnpooledBenchRequest(jw)
					populateBenchRequest(rq, tags)
					recycleUnpooledBenchRequest(jw, rq)
				}
				benchRequestSink = rq
			})
		})
	}
}

func benchmarkSyncSubscription(b *testing.B, jw *Jaws) {
	b.Helper()
	for i := 0; i <= cap(jw.subCh); i++ {
		select {
		case jw.subCh <- subscription{}:
		case <-jw.Done():
			b.Fatal("jaws closed before subscription barrier")
		}
	}
}

func benchmarkWaitClosedSubscription(b *testing.B, jw *Jaws, msgCh chan wire.Message) {
	b.Helper()
	select {
	case _, ok := <-msgCh:
		if ok {
			b.Fatal("subscription channel received a message instead of closing")
		}
	case <-jw.Done():
		b.Fatal("jaws closed before subscription channel")
	}
}

// BenchmarkSubscriptionChannels compares the real subscribe/unsubscribe
// lifecycle using unbuffered production channels against the former one-element
// buffered channels.
func BenchmarkSubscriptionChannels(b *testing.B) {
	for _, c := range []struct {
		name string
		size int
	}{
		{"unbuffered", 0},
		{"buffered=1", 1},
	} {
		b.Run(c.name, func(b *testing.B) {
			jw, err := New()
			if err != nil {
				b.Fatal(err)
			}
			jw.subCh = make(chan subscription, c.size)
			jw.unsubCh = make(chan chan wire.Message, c.size)
			doneCh := make(chan struct{})
			go func() {
				jw.ServeWithTimeout(time.Hour)
				close(doneCh)
			}()
			benchmarkSyncSubscription(b, jw)
			defer func() {
				jw.Close()
				<-doneCh
			}()

			rq := jw.NewRequest(nil)
			defer jw.recycle(rq)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				msgCh := jw.subscribe(rq, 1)
				if msgCh == nil {
					b.Fatal("subscribe returned nil")
				}
				benchmarkSyncSubscription(b, jw)
				jw.unsubscribe(msgCh)
				benchmarkWaitClosedSubscription(b, jw, msgCh)
			}
		})
	}
}

// BenchmarkDistributeDirt guards the per-updateTicker fan-out path: under jw.mu it
// snapshots and order-sorts the dirty tag set and appends it to every active
// Request. The benchmark exists to catch an accidental O(n^2) or per-call
// allocation regression as request and dirty-tag counts grow. The per-iteration
// setDirty repopulation and todoDirt reset are excluded from the timer so only
// distributeDirt is measured.
func BenchmarkDistributeDirt(b *testing.B) {
	for _, c := range []struct{ reqs, tags int }{
		{10, 10}, {100, 100}, {1000, 100}, {100, 1000},
	} {
		b.Run("reqs="+strconv.Itoa(c.reqs)+"/tags="+strconv.Itoa(c.tags), func(b *testing.B) {
			jw, err := New()
			if err != nil {
				b.Fatal(err)
			}
			defer jw.Close()

			reqs := make([]*Request, c.reqs)
			jw.mu.Lock()
			for i := range reqs {
				rq := &Request{Jaws: jw}
				reqs[i] = rq
				jw.requests[key.Key(i+1)] = rq
			}
			jw.mu.Unlock()

			tags := make([]any, c.tags)
			for i := range tags {
				tags[i] = tag.Tag(strconv.Itoa(i))
			}

			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				b.StopTimer()
				jw.setDirty(tags)
				for _, rq := range reqs {
					rq.todoDirt = rq.todoDirt[:0]
				}
				b.StartTimer()
				jw.distributeDirt()
			}
		})
	}
}

// BenchmarkRequestWantMessage guards the per-subscriber cost mustBroadcast pays on
// every broadcast: matching a message destination against a Request's tag map.
// wantMessage is invoked once per subscribed Request per broadcast.
func BenchmarkRequestWantMessage(b *testing.B) {
	const ntags = 100
	jw, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer jw.Close()

	rq := &Request{Jaws: jw, tagMap: map[any][]*Element{}}
	for i := 0; i < ntags; i++ {
		rq.tagMap[tag.Tag(strconv.Itoa(i))] = nil
	}
	hit := tag.Tag(strconv.Itoa(ntags - 1))

	b.Run("single-tag", func(b *testing.B) {
		msg := wire.Message{Dest: hit, What: what.Update}
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_ = rq.wantMessage(&msg)
		}
	})
	b.Run("multi-tag", func(b *testing.B) {
		msg := wire.Message{Dest: []any{tag.Tag("nope"), hit}, What: what.Update}
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_ = rq.wantMessage(&msg)
		}
	})
}

// BenchmarkGetSendMsgs measures the process loop's outbound-queue drain, which it
// invokes at least twice per iteration. The common case is an idle drain (empty
// wsQueue) on a page with many Elements: building the valid-Jid set only when the
// queue actually holds element-targeted messages should allocate nothing here.
func BenchmarkGetSendMsgs(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run("idle/elems="+strconv.Itoa(n), func(b *testing.B) {
			rq := newBenchRequest(b, n)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = rq.getSendMsgs()
			}
		})
	}
}

// BenchmarkGetElementByJid measures resolving a Jid against rq.elems, done per
// inbound event and per removed child id. rq.elems is sorted ascending by Jid, so
// this should scale sub-linearly with element count. "last" looks up the final
// element (the linear-scan worst case); "miss" looks up an absent Jid.
func BenchmarkGetElementByJid(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		rq := newBenchRequest(b, n)
		last := Jid(n)
		miss := Jid(n + 1)
		b.Run("last/elems="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = rq.getElementByJidLocked(last)
			}
		})
		b.Run("miss/elems="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = rq.getElementByJidLocked(miss)
			}
		})
	}
}

// BenchmarkAppendJSQuote measures the tail-script attribute/value quoter. "plain"
// (no '<') is the common case and should append straight into the reused buffer
// with no allocation; "with-bracket" still needs the script-breakout escape.
func BenchmarkAppendJSQuote(b *testing.B) {
	for _, c := range []struct{ name, s string }{
		{"plain", "max-width: 100%; color: rebeccapurple"},
		{"with-bracket", "a < b && c > d </script>"},
	} {
		b.Run(c.name, func(b *testing.B) {
			buf := make([]byte, 0, 256)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf = appendJSQuote(buf[:0], c.s)
			}
			_ = buf
		})
	}
}

// benchSink consumes drained messages so the outbound loop is not eliminated.
var benchSink wire.WsMsg

// BenchmarkSendQueue measures one outbound drain cycle: K element messages are
// queued, then Request.sendQueue drains them through getSendMsgs into the
// WebSocket send channel, which the benchmark then empties. It guards the per-drain
// cost the process loop pays (twice per loop iteration) on the outbound path. rq
// comes from jw.NewRequest(nil) for a live, never-cancelled ctx, and the channel is
// buffered to K so sends never block.
func BenchmarkSendQueue(b *testing.B) {
	for _, k := range []int{8, 64, 512} {
		b.Run("msgs="+strconv.Itoa(k), func(b *testing.B) {
			jw, err := New()
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { jw.Close() })
			rq := jw.NewRequest(nil)
			msgs := make([]wire.WsMsg, k)
			for i := range msgs {
				msgs[i] = wire.WsMsg{Jid: 0, Data: "x"}
			}
			ch := make(chan wire.WsMsg, k)
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				rq.muQueue.Lock()
				rq.wsQueue = append(rq.wsQueue[:0], msgs...)
				rq.muQueue.Unlock()
				rq.sendQueue(ch)
				for len(ch) > 0 {
					benchSink = <-ch
				}
			}
		})
	}
}

// benchDirtSink consumes the sorted []any so the work is not eliminated.
var benchDirtSink []any

// BenchmarkDistributeDirtSort measures sortedDirtTags, the dirty-set ordering
// distributeDirt performs under jw.mu, isolated from the per-request fan-out that
// dominates (and hides it in) BenchmarkDistributeDirt. The cost grows with the
// number of distinct dirty tags marked between update ticks.
func BenchmarkDistributeDirtSort(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		m := make(map[any]int, n)
		for i := 0; i < n; i++ {
			m[tag.Tag(strconv.Itoa(i))] = i
		}
		b.Run("tags="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchDirtSink = sortedDirtTags(m)
			}
		})
	}
}

// BenchmarkRequestMarkWritten measures the per-write cost of recording the write
// instant, which RequestWriter.Write incurs on every write during initial render.
// It runs in parallel because the realistic load is many Requests rendering at
// once; the operation must stay cheap and allocation-free under contention.
func BenchmarkRequestMarkWritten(b *testing.B) {
	jw, err := New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { jw.Close() })
	rq := &Request{Jaws: jw}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rq.MarkWritten()
		}
	})
}
