package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var initialPayloadSink int

func tailEndpoint(body string) (path string, ok bool) {
	const prefix = "/jaws/.tail/"
	if start := strings.Index(body, prefix); start >= 0 {
		if end := strings.IndexByte(body[start:], '"'); end >= 0 {
			path = body[start : start+end]
			ok = true
		}
	}
	return
}

// BenchmarkInitialPageAndTail measures the complete initial HTML response and
// the wrapper-attribute fixup payload fetched before the WebSocket connects.
func BenchmarkInitialPageAndTail(b *testing.B) {
	logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(logger)

	stop := errors.New("benchmark complete")
	var pageBytes, tailBytes int64
	err := run(func(_ string, handler http.Handler) error {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			page := httptest.NewRecorder()
			handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
			path, ok := tailEndpoint(page.Body.String())
			if !ok {
				b.Fatal("initial page is missing the TailHTML endpoint")
			}

			tail := httptest.NewRecorder()
			handler.ServeHTTP(tail, httptest.NewRequest(http.MethodGet, path, nil))
			if tail.Code != http.StatusOK {
				b.Fatalf("TailHTML status = %d, want %d", tail.Code, http.StatusOK)
			}

			pageBytes += int64(page.Body.Len())
			tailBytes += int64(tail.Body.Len())
			initialPayloadSink = page.Body.Len() + tail.Body.Len()

			b.StopTimer()
			requestKey := strings.TrimPrefix(path, "/jaws/.tail/")
			noscript := httptest.NewRecorder()
			handler.ServeHTTP(noscript, httptest.NewRequest(http.MethodGet, "/jaws/"+requestKey+"/noscript", nil))
			if noscript.Code != http.StatusNoContent {
				b.Fatalf("noscript cleanup status = %d, want %d", noscript.Code, http.StatusNoContent)
			}
			b.StartTimer()
		}
		b.StopTimer()
		return stop
	})
	if !errors.Is(err, stop) {
		b.Fatal(err)
	}
	b.ReportMetric(float64(pageBytes)/float64(b.N), "page-B/op")
	b.ReportMetric(float64(tailBytes)/float64(b.N), "tail-B/op")
}
