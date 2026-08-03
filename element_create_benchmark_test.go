package jaws

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// This file holds the cross-version element-creation benchmark and nothing else, so it can
// be copied onto the base commit unchanged: it measures what every Element pays for the
// widget state slot, whether or not that Element ever uses one.

// benchCreateUI is a stateless widget that never touches the state slot, so the measurement
// is the per-Element cost rather than any Template bookkeeping. It documents support for
// multiple live Elements because one value backs every Element the benchmark creates.
type benchCreateUI struct{}

func (benchCreateUI) JawsRender(elem *Element, w io.Writer, params []any) error {
	_, err := io.WriteString(w, "<span>x</span>")
	return err
}

func (benchCreateUI) JawsUpdate(elem *Element) {}

// BenchmarkElementCreateBatch creates and renders a fixed batch of Elements per iteration,
// then deletes them with the timer stopped.
//
// Batching is deliberate: b.StopTimer and b.StartTimer each call runtime.ReadMemStats, so
// toggling around a single sub-microsecond creation would leave the timed section tiny,
// calibration would pick an enormous b.N, and the excluded setup would run for minutes.
// Amortising both calls over the batch keeps that honest, and deleting the batch keeps the
// Request registry bounded instead of growing across iterations. The reported figure is per
// batch, so read it only as a base-versus-new comparison.
func BenchmarkElementCreateBatch(b *testing.B) {
	b.ReportAllocs()
	const batch = 64

	jw, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		b.Fatal("nil request")
	}
	var ui benchCreateUI
	elems := make([]*Element, 0, batch)

	for range b.N {
		elems = elems[:0]
		for range batch {
			elem := rq.NewElement(ui)
			if err := elem.JawsRender(io.Discard, nil); err != nil {
				b.Fatal(err)
			}
			elems = append(elems, elem)
		}
		b.StopTimer()
		rq.DeleteElements(elems)
		b.StartTimer()
	}
}
