package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
)

type benchmarkJsVarState struct {
	Value int `json:"value"`
}

type benchmarkJsVarSlice struct {
	Items []string `json:"items"`
}

func (state *benchmarkJsVarState) JawsSetPath(_ *jaws.Element, _ string, value any) (err error) {
	state.Value = value.(int)
	return
}

func newBenchmarkJsVar(b *testing.B) (jsvar *JsVar[benchmarkJsVarState], elem *jaws.Element) {
	b.Helper()

	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(jw.Close)
	go jw.Serve()

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		b.Fatal("nil request")
	}
	var mu sync.Mutex
	state := benchmarkJsVarState{}
	jsvar = NewJsVar(&mu, &state)
	elem = rq.NewElement(jsvar)
	if err = jsvar.JawsRender(elem, io.Discard, []any{"bench"}); err != nil {
		b.Fatal(err)
	}
	return
}

func BenchmarkJsVarSetPathBroadcast(b *testing.B) {
	jsvar, elem := newBenchmarkJsVar(b)

	b.Run("Serial", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := jsvar.JawsSetPath(elem, "value", i); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := jsvar.JawsSetPath(elem, "value", 1); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func BenchmarkJsVarPathSetterMutation(b *testing.B) {
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(jw.Close)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		b.Fatal("nil request")
	}
	var mu sync.Mutex
	state := benchmarkJsVarState{}
	jsvar := NewJsVar(&mu, &state)
	elem := rq.NewElement(jsvar)

	b.Run("Serial", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := jsvar.JawsSetPath(elem, "value", i); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := jsvar.JawsSetPath(elem, "value", 1); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// BenchmarkJsVarClientWrite guards against reintroducing per-write full-value
// marshaling (the O(n^2) append-flood hazard) in the client input path. It
// overwrites a single element so the total serialized size stays fixed while the
// backing slice is either tiny or large. With the running size accounting the
// per-write cost is independent of the slice length, so SmallState and LargeState
// report nearly the same ns/op; a regression that marshals the whole value on every
// write would make LargeState scale with the slice length.
func BenchmarkJsVarClientWrite(b *testing.B) {
	old := MaxClientJsVarBytes
	MaxClientJsVarBytes = 1 << 30 // effectively unbounded: accounting runs but never confirms
	b.Cleanup(func() { MaxClientJsVarBytes = old })

	run := func(b *testing.B, initial int) {
		jw, err := jaws.New()
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(jw.Close)
		go jw.Serve()
		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		if rq == nil {
			b.Fatal("nil request")
		}
		var mu sync.Mutex
		state := benchmarkJsVarSlice{Items: make([]string, initial)}
		for i := range state.Items {
			state.Items[i] = "0123456789"
		}
		jsvar := NewJsVar(&mu, &state)
		elem := rq.NewElement(jsvar)
		if err = jsvar.JawsRender(elem, io.Discard, []any{"bench"}); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Overwrite the first element with a distinct value; the total size stays
			// fixed, so per-write cost should not depend on the slice length.
			if err = jsvar.JawsInput(elem, "items.0="+strconv.Quote(strconv.Itoa(i))); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.Run("SmallState", func(b *testing.B) { run(b, 1) })
	b.Run("LargeState", func(b *testing.B) { run(b, 10000) })
}

func BenchmarkValidateJsVarName(b *testing.B) {
	params := []any{"state"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		name, err := validateJsVarName(params)
		if err != nil || name != "state" {
			b.Fatalf("validateJsVarName() = %q, %v", name, err)
		}
	}
}
