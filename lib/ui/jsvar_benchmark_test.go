package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
)

type benchmarkJsVarState struct {
	Value int `json:"value"`
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
