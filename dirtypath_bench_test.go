package jaws

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/linkdata/jaws/lib/tag"
)

func BenchmarkDirtyPathFanout(b *testing.B) {
	for _, bindings := range []int{1, 8} {
		for _, phase := range []string{"total", "distribute"} {
			b.Run("bindings="+strconv.Itoa(bindings)+"/phase="+phase, func(b *testing.B) {
				jw, err := New()
				if err != nil {
					b.Fatal(err)
				}
				jw.updateTicker.Stop()
				serveDone := make(chan struct{})
				go func() {
					jw.Serve()
					close(serveDone)
				}()
				b.Cleanup(func() {
					jw.Close()
					<-serveDone
				})
				select {
				case jw.subCh <- subscription{}:
				case <-time.After(time.Second):
					b.Fatal("serve loop did not start")
				}
				const requests = 100
				selector := tag.Tag("shared")
				reqs := make([]*Request, requests)
				for i := range reqs {
					rq := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
					for range bindings {
						rq.NewElement(pathUpdateUI{}).Tag(selector)
					}
					reqs[i] = rq
				}
				b.ReportAllocs()
				for b.Loop() {
					b.StopTimer()
					jw.DirtyPath(selector, "value")
					b.StartTimer()
					jw.distributeDirt()
					if phase == "distribute" {
						b.StopTimer()
					}
					for _, rq := range reqs {
						if got := len(rq.makePathUpdateList()); got != bindings {
							b.Fatalf("path targets = %d, want %d", got, bindings)
						}
					}
					if phase == "distribute" {
						b.StartTimer()
					}
				}
			})
		}
	}
}
