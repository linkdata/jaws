package jaws

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestJaws_ErrorCountConcurrentReports(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	first := jw.newRequest(httptest.NewRequest(http.MethodGet, "/first", nil))
	second := jw.newRequest(httptest.NewRequest(http.MethodGet, "/second", nil))
	firstElem := first.NewElement(new(testUi))
	secondElem := second.NewElement(new(testUi))
	firstElem.Tag(jw.ErrorCountTag())
	secondElem.Tag(jw.ErrorCountTag())
	jw.StatusMetrics.Store(StatusMetricErrors)
	jw.maintenance(time.Hour)
	if got := jw.distributeDirt(); got != 1 {
		t.Fatalf("initial sample distributed %d selectors, want 1", got)
	}
	requireUpdateList(t, first, firstElem)
	requireUpdateList(t, second, secondElem)

	wantErr := errors.New("concurrent report")
	const workers = 16
	const reportsPerWorker = 64
	var wg sync.WaitGroup
	wg.Add(workers)
	for worker := range workers {
		go func() {
			defer wg.Done()
			for report := range reportsPerWorker {
				if (worker+report)%2 == 0 {
					_ = jw.Log(wantErr)
				} else {
					_ = first.Log(wantErr)
				}
			}
		}()
	}
	wg.Wait()

	want := uint64(workers * reportsPerWorker)
	if got := jw.ErrorCount(); got != want {
		t.Fatalf("ErrorCount() = %d, want %d", got, want)
	}
	jw.maintenance(time.Hour)
	if got := jw.distributeDirt(); got != 1 {
		t.Fatalf("distributed selectors = %d, want 1", got)
	}
	requireUpdateList(t, first, firstElem)
	requireUpdateList(t, second, secondElem)
}
