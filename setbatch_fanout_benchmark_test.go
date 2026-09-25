package jaws

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// BenchmarkSetBatchInterleavedFanout measures recipient buffer use for two destinations.
func BenchmarkSetBatchInterleavedFanout(b *testing.B) {
	jw, err := New()
	if err != nil {
		b.Fatal(err)
	}
	serveDone := make(chan struct{})
	go func() {
		jw.Serve()
		close(serveDone)
	}()
	b.Cleanup(func() {
		jw.Close()
		<-serveDone
	})

	a, c := tag.Tag("A"), tag.Tag("B")
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rq.NewElement(&testUi{}).Tag(a)
	rq.NewElement(&testUi{}).Tag(c)
	msgCh := jw.subscribe(rq, 64)
	msgs := make([]wire.Message, 0, 32)
	for i := range 16 {
		data := "f" + strconv.Itoa(i) + "=" + strconv.Itoa(i)
		msgs = append(
			msgs,
			wire.Message{Dest: a, What: what.Set, Data: data},
			wire.Message{Dest: c, What: what.Set, Data: data},
		)
	}

	b.ReportAllocs()
	b.ResetTimer()
	var broadcasts int
	for b.Loop() {
		for _, msg := range msgs {
			jw.Broadcast(msg)
		}
		jw.Broadcast(wire.Message{What: what.Alert, Data: "barrier"})
		for {
			msg := <-msgCh
			if msg.What == what.Alert {
				break
			}
			broadcasts++
		}
	}
	b.ReportMetric(float64(broadcasts)/float64(b.N*len(msgs)), "broadcasts/path")
}
