package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	jawstag "github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
)

var dirtyFanoutSink int

// BenchmarkSingleCellDirtyFanout measures the per-event work a single-cell action
// triggers: how many element re-renders a flag toggle's dirty set resolves to after
// tag expansion. It renders a full default board (100 cell elements registered under
// both their per-cell tag and the shared board tag), then resolves the dirty tags
// the way the framework's update step does (expand, then look up elements per tag).
//
// This is the cost the fix targets: before, a single-cell dirty expanded to the
// shared board tag and resolved to all 100 cells; after, it resolves to one. Run
// with -benchmem and compare before/after with benchstat.
func BenchmarkSingleCellDirtyFanout(b *testing.B) {
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	defer jw.Close()

	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		b.Fatal("expected a request")
	}

	g := newGame(10, 10, 15)
	for _, row := range g.Board() {
		for _, cell := range row {
			elem := rq.NewElement(ui.NewButton(cell))
			var sb strings.Builder
			if err := elem.JawsRender(&sb, []any{cell.BoardTag(), `class="cell"`}); err != nil {
				b.Fatal(err)
			}
		}
	}

	cell := g.cells[0][0]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Mirror the framework's dirty dispatch: expand the toggle's tags and
		// resolve each to its registered elements (Request.GetElements is the same
		// tagMap lookup makeUpdateList performs). The sum is the number of element
		// re-renders the toggle would drive.
		expanded, err := jawstag.TagExpand(nil, g.toggleFlag(cell))
		if err != nil {
			b.Fatal(err)
		}
		n := 0
		for _, tg := range expanded {
			n += len(rq.GetElements(tg))
		}
		dirtyFanoutSink = n
	}
}
