package jaws

import (
	"strconv"
	"strings"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// BenchmarkRequestIncomingRemoveCleanup compares the batched removal report sent
// by jaws.js with the same live Jids split across attacker-controlled reports.
func BenchmarkRequestIncomingRemoveCleanup(b *testing.B) {
	benchmarks := []struct {
		elements int
		split    bool
	}{
		{elements: 1000},
		{elements: 3000},
		{elements: 100, split: true},
		{elements: 1000, split: true},
		{elements: 3000, split: true},
		{elements: 10000, split: true},
	}

	for _, benchmark := range benchmarks {
		name := "batch"
		if benchmark.split {
			name = "split"
		}
		b.Run(name+"/elements="+strconv.Itoa(benchmark.elements), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				b.StopTimer()
				jw, err := New()
				if err != nil {
					b.Fatal(err)
				}
				rq := &Request{Jaws: jw, tagMap: make(map[any][]*Element)}
				container := rq.NewElement(nil)
				ids := make([]string, benchmark.elements)
				for i := range ids {
					ids[i] = rq.NewElement(nil).Jid().String()
				}
				tagValue := tag.Tag("remove-benchmark")
				rq.tagMap[tagValue] = append([]*Element(nil), rq.elems[1:]...)
				data := strings.Join(ids, "\t")
				msg := wire.WsMsg{Jid: container.Jid(), What: what.Remove, Data: data}
				if !benchmark.split {
					if size := len(msg.Append(nil)); size > webSocketReadLimit {
						b.Fatalf("batched Remove frame = %d bytes, limit %d", size, webSocketReadLimit)
					}
				}

				b.StartTimer()
				if benchmark.split {
					for _, id := range ids {
						msg.Data = id
						rq.handleIncoming(msg, nil)
					}
				} else {
					rq.handleIncoming(msg, nil)
				}
				b.StopTimer()

				if got := rq.GetElementByJid(container.Jid()); got != container {
					b.Fatalf("container lookup = %p, want %p", got, container)
				}
				if got := rq.GetElementByJid(2); got != nil {
					b.Fatalf("first removed element remains registered: %v", got)
				}
				if got := rq.GetElements(tagValue); len(got) != 0 {
					b.Fatalf("removed tag retains %d elements", len(got))
				}
				jw.Close()
			}
		})
	}
}
