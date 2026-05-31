package jaws

import (
	"strconv"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

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
				jw.requests[uint64(i+1)] = rq
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
