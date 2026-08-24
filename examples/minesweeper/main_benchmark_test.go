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

// BenchmarkSingleCellDirtyFanout measures tag expansion and cell-Element lookup
// for one flag action on a full default board.
//
// It deliberately registers only cell Buttons, isolating cell fanout from the
// separate Stats update. TestSingleCellDirtyStaysScopedToOneCell guards the
// corresponding correctness invariant.
func BenchmarkSingleCellDirtyFanout(b *testing.B) {
	jw, err := jaws.New()
	if err != nil {
		b.Fatal(err)
	}
	defer jw.Close()
	go jw.Serve()

	rq := jw.NewRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	g := newGame(10, 10, 15)
	for _, row := range g.Board() {
		for _, current := range row {
			elem := rq.NewElement(ui.NewButton(current))
			var sb strings.Builder
			if err := elem.JawsRender(&sb, cellButtonParams(current)); err != nil {
				b.Fatal(err)
			}
		}
	}

	cell := g.cells[0][0]
	b.ReportAllocs()
	for b.Loop() {
		expanded, err := jawstag.TagExpand(g.toggleFlag(cell))
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
