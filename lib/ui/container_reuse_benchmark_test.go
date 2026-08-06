package ui

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
)

// This file isolates benchmarks for the container's two child-reuse paths: equal rebuilt
// values and stable values returned unchanged.

const benchReuseTemplate = `{{define "bench-row"}}<span>row</span>{{end}}`

// benchReuseRow is a comparable dot.
type benchReuseRow struct{ id int }

// benchRebuildingContainer builds its children afresh on every call, the way an
// application container does. Reuse therefore depends on those values comparing equal.
type benchRebuildingContainer struct {
	rows  []*benchReuseRow
	build func(*benchReuseRow) jaws.UI
}

func (c *benchRebuildingContainer) JawsContains(elem *jaws.Element) (contents []jaws.UI) {
	contents = make([]jaws.UI, 0, len(c.rows))
	for _, row := range c.rows {
		contents = append(contents, c.build(row))
	}
	return
}

// benchStableContainer hands back the same child values every call.
type benchStableContainer struct{ contents []jaws.UI }

func (c *benchStableContainer) JawsContains(elem *jaws.Element) []jaws.UI { return c.contents }

func benchReuseRequest(b *testing.B) (*jaws.Jaws, *jaws.Request) {
	b.Helper()
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	if err = jw.AddTemplateLookuper(template.Must(template.New("bench").Parse(benchReuseTemplate))); err != nil {
		jw.Close()
		b.Fatal(err)
	}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		jw.Close()
		b.Fatal("nil request")
	}
	return jw, rq
}

// BenchmarkContainerOfTemplatesUpdate times one update of a container whose children are
// rebuilt on every JawsContains call and whose collection has not changed. When the child
// values compare equal the container reuses their Elements and the update is nearly free;
// when they do not, it removes and re-appends the whole collection.
func BenchmarkContainerOfTemplatesUpdate(b *testing.B) {
	b.ReportAllocs()
	const rows = 200
	for range b.N {
		b.StopTimer()
		jw, rq := benchReuseRequest(b)
		tc := &benchRebuildingContainer{
			build: func(row *benchReuseRow) jaws.UI { return NewTemplate("div", "bench-row", row) },
		}
		for i := range rows {
			tc.rows = append(tc.rows, &benchReuseRow{id: i})
		}
		container := NewContainer("div", tc)
		elem := rq.NewElement(container)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		container.JawsUpdate(elem)
		b.StopTimer()
		jw.Close()
	}
}

// BenchmarkContainerOfStableChildrenUpdate measures updates whose child values are returned
// unchanged, guarding the identity-based reuse path alongside equal rebuilt values.
func BenchmarkContainerOfStableChildrenUpdate(b *testing.B) {
	b.ReportAllocs()
	const rows = 200
	for range b.N {
		b.StopTimer()
		jw, rq := benchReuseRequest(b)
		tc := &benchStableContainer{}
		for i := range rows {
			tc.contents = append(tc.contents, NewSpan(&benchReuseRow{id: i}))
		}
		container := NewContainer("div", tc)
		elem := rq.NewElement(container)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		container.JawsUpdate(elem)
		b.StopTimer()
		jw.Close()
	}
}

// BenchmarkContainerInitialRender measures the state-allocation path separately from
// the larger child-reuse benchmarks.
func BenchmarkContainerInitialRender(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		jw, rq := benchReuseRequest(b)
		bc := &benchStableContainer{contents: benchChildren(0, 4)}
		container := NewContainer("div", bc)
		elem := rq.NewElement(container)
		b.StartTimer()
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		jw.Close()
	}
}

// BenchmarkContainerUnchangedUpdate measures a small reconciliation that reuses every
// child and emits no browser mutation.
func BenchmarkContainerUnchangedUpdate(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		jw, rq := benchReuseRequest(b)
		bc := &benchStableContainer{contents: benchChildren(0, 4)}
		container := NewContainer("div", bc)
		elem := rq.NewElement(container)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		container.JawsUpdate(elem)
		b.StopTimer()
		jw.Close()
	}
}

// BenchmarkContainerAppendUpdate measures a small reconciliation that appends one
// child.
func BenchmarkContainerAppendUpdate(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		jw, rq := benchReuseRequest(b)
		bc := &benchStableContainer{contents: benchChildren(0, 3)}
		container := NewContainer("div", bc)
		elem := rq.NewElement(container)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		bc.contents = benchChildren(0, 4)
		b.StartTimer()
		container.JawsUpdate(elem)
		b.StopTimer()
		jw.Close()
	}
}

// BenchmarkContainerRemoveUpdate measures a small reconciliation that removes one
// child.
func BenchmarkContainerRemoveUpdate(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		jw, rq := benchReuseRequest(b)
		bc := &benchStableContainer{contents: benchChildren(0, 4)}
		container := NewContainer("div", bc)
		elem := rq.NewElement(container)
		if err := elem.JawsRender(io.Discard, nil); err != nil {
			b.Fatal(err)
		}
		bc.contents = benchChildren(0, 3)
		b.StartTimer()
		container.JawsUpdate(elem)
		b.StopTimer()
		jw.Close()
	}
}

// BenchmarkContainerRegisterFirstUpdate measures the lazy state-claim path used by an
// update-only Register Element.
func BenchmarkContainerRegisterFirstUpdate(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		jw, rq := benchReuseRequest(b)
		bc := &benchStableContainer{contents: benchChildren(0, 4)}
		container := NewContainer("div", bc)
		elem := rq.NewElement(NewRegister(container))
		b.StartTimer()
		elem.JawsUpdate()
		b.StopTimer()
		jw.Close()
	}
}
