package jaws

import (
	"strconv"
	"testing"
	"time"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// BenchmarkRequestQueueDelivery measures queue-to-outbound delivery.
//
// Each parallel worker owns one live Request loop; socket I/O is excluded.
func BenchmarkRequestQueueDelivery(b *testing.B) {
	jw := newBenchPoolJaws(b)
	jw.updateTicker.Stop()
	go jw.ServeWithTimeout(time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rq := NewTestRequest(jw, nil)
		if rq == nil {
			b.Error("request setup failed")
			return
		}
		defer func() {
			rq.Close()
			<-rq.DoneCh
		}()
		<-rq.ReadyCh
		msg := wire.WsMsg{What: what.Alert, Data: "queued"}
		for pb.Next() {
			rq.queue(msg)
			if got := <-rq.OutCh; got != msg {
				b.Errorf("output = %v, want %v", got, msg)
				return
			}
		}
	})
}

// BenchmarkRequestMaintenance measures maintenance over idle running Requests.
//
// No transport or processing goroutines compete with the scan.
func BenchmarkRequestMaintenance(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run("requests="+strconv.Itoa(n), func(b *testing.B) {
			jw := newBenchPoolJaws(b)
			for range n {
				rq := jw.newRequest(nil)
				jw.mu.Lock()
				rq.mu.Lock()
				rq.storeState(reqRunning)
				rq.mu.Unlock()
				jw.mu.Unlock()
			}
			b.ReportAllocs()
			for b.Loop() {
				jw.maintenance(time.Hour)
			}
			if got := jw.RequestCount(); got != n {
				b.Fatalf("request count = %d, want %d", got, n)
			}
		})
	}
}

// BenchmarkRequestDirtyFanout measures dirty-tag assignment to pending Requests.
//
// Each iteration sorts and assigns ten tags to 100 Requests, including
// replenishing the dirty set. No process loops run; pending wake signals remain
// coalesced between iterations.
func BenchmarkRequestDirtyFanout(b *testing.B) {
	jw := newBenchPoolJaws(b)
	reqs := make([]*Request, 100)
	for i := range reqs {
		reqs[i] = jw.newRequest(nil)
	}
	tags := make([]any, 10)
	for i := range tags {
		tags[i] = tag.Tag(strconv.Itoa(i))
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, rq := range reqs {
			rq.todoDirt = rq.todoDirt[:0]
		}
		jw.setDirty(tags)
		jw.distributeDirt()
	}
	for _, rq := range reqs {
		if got := len(rq.todoDirt); got != len(tags) {
			b.Fatalf("assigned tags = %d, want %d", got, len(tags))
		}
	}
}
