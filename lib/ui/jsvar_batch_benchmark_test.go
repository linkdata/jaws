package ui

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// BenchmarkJsVarBurstFanout measures the Set frames emitted to two requests
// while one browser repeatedly writes the same JsVar path.
func BenchmarkJsVarBurstFanout(b *testing.B) {
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	go jw.Serve()
	first := jawstest.NewTestRequest(jw, nil)
	second := jawstest.NewTestRequest(jw, nil)
	<-first.ReadyCh
	<-second.ReadyCh
	var readers sync.WaitGroup
	defer func() {
		first.Cancel(nil)
		second.Cancel(nil)
		first.Close()
		second.Close()
		<-first.DoneCh
		<-second.DoneCh
		readers.Wait()
		jw.Close()
	}()

	var mu sync.Mutex
	state := struct {
		Value int `json:"value"`
	}{}
	firstJsVar := NewJsVar(&mu, &state)
	secondJsVar := NewJsVar(&mu, &state)
	firstElem := first.NewElement(firstJsVar)
	secondElem := second.NewElement(secondJsVar)
	if err = firstElem.JawsRender(io.Discard, []any{"state"}); err != nil {
		b.Fatal(err)
	}
	if err = secondElem.JawsRender(io.Discard, []any{"state"}); err != nil {
		b.Fatal(err)
	}

	var frames atomic.Int64
	barrier := make(chan struct{}, 2)
	for _, tr := range []*jawstest.TestRequest{first, second} {
		readers.Add(1)
		go func(tr *jawstest.TestRequest) {
			defer readers.Done()
			for msg := range tr.OutCh {
				switch msg.What {
				case what.Set:
					frames.Add(1)
				case what.Alert:
					barrier <- struct{}{}
				}
			}
		}(tr)
	}
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		if err = firstJsVar.JawsInput(firstElem, fmt.Sprintf("value=%d", i)); err != nil {
			b.Fatal(err)
		}
		// Keep the baseline request below its outbound capacity while still
		// writing much faster than the regular update interval.
		time.Sleep(time.Millisecond)
	}
	b.StopTimer()

	time.Sleep(2 * jaws.DefaultUpdateInterval)
	jw.Broadcast(wire.Message{What: what.Alert, Data: "barrier"})
	for range 2 {
		select {
		case <-barrier:
		case <-time.After(10 * time.Second):
			b.Fatal("timed out draining JsVar broadcast frames")
		}
	}
	b.ReportMetric(float64(frames.Load())/float64(b.N), "set-frames/op")
}
