package ui

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
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

// BenchmarkJsVarClientWrite measures both the default unchecked path and the
// explicitly enabled full-value JSON size check. Each case alternates equal-length
// values in one element while the backing slice is tiny or large. The unchecked
// path should be insensitive to slice length; the checked path measures the
// intentionally size-dependent full-value encoding cost.
func BenchmarkJsVarClientWrite(b *testing.B) {
	run := func(b *testing.B, initial int, checked bool) {
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
		if checked {
			jsvar.ClientCheck = JSONSizeCheck[benchmarkJsVarSlice](math.MaxInt)
		}
		elem := rq.NewElement(jsvar)
		if err = jsvar.JawsRender(elem, io.Discard, []any{"bench"}); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			value := `"abcdefghij"`
			if i&1 != 0 {
				value = `"0123456789"`
			}
			if err = jsvar.JawsInput(elem, "items.0="+value); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.Run("SmallState", func(b *testing.B) { run(b, 1, false) })
	b.Run("LargeState", func(b *testing.B) { run(b, 10000, false) })
	b.Run("JSONSizeCheck", func(b *testing.B) {
		b.Run("SmallState", func(b *testing.B) { run(b, 1, true) })
		b.Run("LargeState", func(b *testing.B) { run(b, 10000, true) })
	})
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
