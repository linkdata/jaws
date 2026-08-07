package ui

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
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
	b.StopTimer()
	tr := newReuseRequest(b)
	bc := &benchStableContainer{contents: benchChildren(0, 4)}
	container := NewContainer("div", bc)
	b.ReportAllocs()
	b.ResetTimer()
	for completed := 0; completed < b.N; {
		batchSize := min(benchmarkContainerBatchSize, b.N-completed)
		elems := make([]*jaws.Element, batchSize)
		for i := range elems {
			elems[i] = tr.NewElement(container)
		}

		b.StartTimer()
		var renderErr error
		for _, elem := range elems {
			if err := elem.JawsRender(io.Discard, nil); renderErr == nil {
				renderErr = err
			}
		}
		b.StopTimer()
		if renderErr != nil {
			b.Fatal(renderErr)
		}
		for _, elem := range elems {
			benchmarkRequireContainerElementCount(b, elem, 4)
		}
		deleteOwnedElements(tr.Request, elems)
		benchmarkRequireDeletedContainerElements(b, tr.Request, elems)
		completed += batchSize
	}
}

// BenchmarkContainerUnchangedUpdate measures a small reconciliation that reuses every
// child and emits no browser mutation.
func BenchmarkContainerUnchangedUpdate(b *testing.B) {
	b.StopTimer()
	tr := newReuseRequest(b)
	bc := &benchStableContainer{contents: benchChildren(0, 4)}
	container := NewContainer("div", bc)
	elem := tr.NewElement(container)
	if err := elem.JawsRender(io.Discard, nil); err != nil {
		b.Fatal(err)
	}
	before := benchmarkContainerElements(b, elem, 4)
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for range b.N {
		container.JawsUpdate(elem)
	}
	b.StopTimer()
	after := benchmarkContainerElements(b, elem, 4)
	for i := range before {
		if after[i] != before[i] {
			b.Fatalf("child %d = %v, want retained Element %v", i, after[i].Jid(), before[i].Jid())
		}
	}
	benchmarkRequireContainerWire(b,
		nestedReorderDrainWire(b, tr, "container unchanged benchmark"), elem.Jid())
}

// BenchmarkContainerAppendRemoveUpdate measures alternating small reconciliations
// that append and remove one child. Alternating three and four children makes every
// measured update structural while retaining one request-scoped fixture.
func BenchmarkContainerAppendRemoveUpdate(b *testing.B) {
	b.StopTimer()
	tr := newReuseRequest(b)
	three := benchChildren(0, 3)
	four := benchChildren(0, 4)
	bc := &benchStableContainer{contents: three}
	container := NewContainer("div", bc)
	elem := tr.NewElement(container)
	if err := elem.JawsRender(io.Discard, nil); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	expanded := false
	for completed := 0; completed < b.N; {
		batchSize := min(benchmarkContainerBatchSize, b.N-completed)
		startedExpanded := expanded
		b.StartTimer()
		for range batchSize {
			if expanded {
				bc.contents = three
			} else {
				bc.contents = four
			}
			container.JawsUpdate(elem)
			expanded = !expanded
		}
		b.StopTimer()
		wantCount := 3
		if expanded {
			wantCount = 4
		}
		benchmarkRequireContainerElementCount(b, elem, wantCount)
		benchmarkRequireContainerResizeWire(b,
			nestedReorderDrainWire(b, tr, "container append/remove benchmark"),
			elem.Jid(), batchSize, startedExpanded)
		completed += batchSize
	}
}

// BenchmarkContainerRegisterFirstUpdate measures the lazy state-claim path used by an
// update-only registered Element.
func BenchmarkContainerRegisterFirstUpdate(b *testing.B) {
	b.StopTimer()
	tr := newReuseRequest(b)
	bc := &benchStableContainer{contents: benchChildren(0, 4)}
	container := NewContainer("div", bc)
	b.ReportAllocs()
	b.ResetTimer()
	for completed := 0; completed < b.N; {
		batchSize := min(benchmarkContainerBatchSize, b.N-completed)
		elems := make([]*jaws.Element, batchSize)
		for i := range elems {
			elems[i] = tr.NewElement(registerUI{Updater: container})
		}

		b.StartTimer()
		for _, elem := range elems {
			elem.JawsUpdate()
		}
		b.StopTimer()
		for _, elem := range elems {
			benchmarkRequireContainerElementCount(b, elem, 4)
		}
		benchmarkRequireContainerBatchWire(b,
			nestedReorderDrainWire(b, tr, "container register benchmark"), elems,
			what.Append, what.Append, what.Append, what.Append, what.Order)
		deleteOwnedElements(tr.Request, elems)
		benchmarkRequireDeletedContainerElements(b, tr.Request, elems)
		completed += batchSize
	}
}

// Batching amortizes timer transitions and untimed validation while bounding
// simultaneously live Elements and queued wire operations.
const benchmarkContainerBatchSize = 256

func benchmarkContainerElements(b *testing.B, elem *jaws.Element, want int) (contents []*jaws.Element) {
	b.Helper()
	state, ok := jaws.ElementState(elem).(*containerState)
	if !ok || state == nil {
		b.Fatalf("Element %v state = %T, want *containerState", elem.Jid(), jaws.ElementState(elem))
	}
	state.mu.Lock()
	contents = append(contents, state.contents...)
	state.mu.Unlock()
	if len(contents) != want {
		b.Fatalf("Element %v has %d children, want %d", elem.Jid(), len(contents), want)
	}
	for _, child := range contents {
		if got := elem.Request.GetElementByJid(child.Jid()); got != child {
			b.Fatalf("request Element %v = %p, want retained %p", child.Jid(), got, child)
		}
	}
	return
}

func benchmarkRequireContainerElementCount(b *testing.B, elem *jaws.Element, want int) {
	b.Helper()
	state, ok := jaws.ElementState(elem).(*containerState)
	if !ok || state == nil {
		b.Fatalf("Element %v state = %T, want *containerState", elem.Jid(), jaws.ElementState(elem))
	}
	state.mu.Lock()
	got := len(state.contents)
	state.mu.Unlock()
	if got != want {
		b.Fatalf("Element %v has %d children, want %d", elem.Jid(), got, want)
	}
}

func benchmarkRequireDeletedContainerElements(b *testing.B, rq *jaws.Request, elems []*jaws.Element) {
	b.Helper()
	for _, elem := range elems {
		if got := rq.GetElementByJid(elem.Jid()); got != nil {
			b.Fatalf("cleaned Element %v remains registered", elem.Jid())
		}
	}
}

func benchmarkRequireContainerWire(b *testing.B, got []wire.WsMsg, jid jaws.Jid, want ...what.What) {
	b.Helper()
	if len(got) != len(want) {
		b.Fatalf("wire operations = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, message := range got {
		if message.Jid != jid || message.What != want[i] {
			b.Fatalf("wire operation %d = %v %v, want %v %v", i, message.Jid, message.What, jid, want[i])
		}
	}
}

func benchmarkRequireContainerBatchWire(b *testing.B, got []wire.WsMsg, elems []*jaws.Element, want ...what.What) {
	b.Helper()
	wantCount := len(elems) * len(want)
	if len(got) != wantCount {
		b.Fatalf("wire operations = %d, want %d", len(got), wantCount)
	}
	messageIndex := 0
	for _, elem := range elems {
		for _, wantWhat := range want {
			message := got[messageIndex]
			if message.Jid != elem.Jid() || message.What != wantWhat {
				b.Fatalf("wire operation %d = %v %v, want %v %v", messageIndex, message.Jid, message.What, elem.Jid(), wantWhat)
			}
			messageIndex++
		}
	}
}

func benchmarkRequireContainerResizeWire(b *testing.B, got []wire.WsMsg, jid jaws.Jid, updates int, expanded bool) {
	b.Helper()
	if want := updates * 2; len(got) != want {
		b.Fatalf("wire operations = %d, want %d", len(got), want)
	}
	for updateIndex := range updates {
		wantWhat := what.Append
		if expanded {
			wantWhat = what.Remove
		}
		operation := got[updateIndex*2]
		order := got[updateIndex*2+1]
		if operation.Jid != jid || operation.What != wantWhat {
			b.Fatalf("wire operation %d = %v %v, want %v %v", updateIndex*2, operation.Jid, operation.What, jid, wantWhat)
		}
		if order.Jid != jid || order.What != what.Order {
			b.Fatalf("wire operation %d = %v %v, want %v %v", updateIndex*2+1, order.Jid, order.What, jid, what.Order)
		}
		expanded = !expanded
	}
}
