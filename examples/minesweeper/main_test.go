package main

import (
	"bytes"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	"github.com/linkdata/jaws/lib/bind"
	jawstag "github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func newExampleRequest(t *testing.T) (*jaws.Jaws, *jawstest.TestRequest) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()

	rq := jawstest.NewTestRequest(jw, httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		jw.Close()
		t.Fatal("expected test request")
	}
	t.Cleanup(func() {
		rq.Close()
		jw.Close()
	})
	return jw, rq
}

func tailScript(t *testing.T, jw *jaws.Jaws, rq *jawstest.TestRequest) string {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	jw.ServeHTTP(rr, req)
	return rr.Body.String()
}

func assertTagSetEqual(t *testing.T, got []any, want ...any) {
	t.Helper()
	gotSet := make(map[any]struct{}, len(got))
	for _, v := range got {
		gotSet[v] = struct{}{}
	}
	wantSet := make(map[any]struct{}, len(want))
	for _, v := range want {
		wantSet[v] = struct{}{}
	}
	if !reflect.DeepEqual(gotSet, wantSet) {
		t.Fatalf("tag set mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func findSeedWithSkipFirst(t *testing.T, total, skipIdx int) int64 {
	t.Helper()
	for seed := int64(0); seed < 10_000; seed++ {
		if rand.New(rand.NewSource(seed)).Perm(total)[0] == skipIdx {
			return seed
		}
	}
	t.Fatalf("failed to find seed for total=%d skip=%d", total, skipIdx)
	return 0
}

func TestCellButtonUsesCellTagsAndHandlers(t *testing.T) {
	jw, rq := newExampleRequest(t)

	g := newGame(3, 3, 1)
	cell := g.cells[0][0]
	elem := rq.NewElement(ui.NewButton(cell))

	// Mirror the template, which passes the shared board tag alongside the cell
	// (see index.html: {{$.Button . .BoardTag ...}}). The cell's own JawsGetTag
	// returns only the cell, so the board tag must be supplied as a render param.
	var body bytes.Buffer
	if err := elem.JawsRender(&body, []any{cell.BoardTag(), `class="cell"`}); err != nil {
		t.Fatal(err)
	}
	if !elem.HasTag(cell) {
		t.Fatal("expected cell tag to be registered")
	}
	if !elem.HasTag(&g.cells) {
		t.Fatal("expected shared board tag to be registered")
	}

	if err := jaws.CallEventHandlers(elem.UI(), elem, what.ContextMenu, "0 0 0 flag"); err != nil {
		t.Fatalf("context menu error: %v", err)
	}
	if !cell.flagged {
		t.Fatal("expected cell to be flagged after context menu")
	}
	if g.flags != 1 {
		t.Fatalf("flags = %d, want 1", g.flags)
	}

	other := g.cells[0][1]
	otherElem := rq.NewElement(ui.NewButton(other))
	if err := otherElem.JawsRender(&body, []any{`class="cell"`}); err != nil {
		t.Fatal(err)
	}
	if err := jaws.CallEventHandlers(otherElem.UI(), otherElem, what.Click, "0 0 0 reveal"); err != nil {
		t.Fatalf("click error: %v", err)
	}
	if !g.started {
		t.Fatal("expected first click to start the game")
	}
	if !other.revealed {
		t.Fatal("expected clicked cell to be revealed")
	}

	// Drain attr/class updates. The harness process loop may already have
	// forwarded updates to OutCh before the tail endpoint is fetched.
	if got := tailScript(t, jw, rq); got == "" {
		select {
		case <-rq.OutCh:
		case <-time.After(time.Second):
			t.Fatal("expected queued or forwarded updates")
		}
	}
}

func TestCellViewHTMLAndLabels(t *testing.T) {
	tests := []struct {
		name      string
		view      cellView
		wantHTML  string
		wantLabel string
	}{
		{
			name:      "revealed mine",
			view:      cellView{revealed: true, mine: true},
			wantHTML:  `<span class="glyph glyph-mine">☠</span>`,
			wantLabel: "Mine",
		},
		{
			name:      "revealed adjacent",
			view:      cellView{revealed: true, adjacent: 3},
			wantHTML:  `<span class="cleared">3</span>`,
			wantLabel: "Revealed cell with 3 adjacent mines",
		},
		{
			name:      "revealed empty",
			view:      cellView{revealed: true},
			wantHTML:  `<span class="cleared"></span>`,
			wantLabel: "Revealed empty cell",
		},
		{
			name:      "flagged hidden",
			view:      cellView{flagged: true},
			wantHTML:  `<span class="glyph glyph-flag">⚑</span>`,
			wantLabel: "Flagged hidden cell",
		},
		{
			name:      "hidden game over",
			view:      cellView{gameOver: true},
			wantHTML:  "",
			wantLabel: "Hidden cell, game over",
		},
		{
			name:      "hidden",
			view:      cellView{},
			wantHTML:  "",
			wantLabel: "Hidden cell",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.view.HTML()); got != tt.wantHTML {
				t.Fatalf("HTML() = %q, want %q", got, tt.wantHTML)
			}
			if got := tt.view.label(); got != tt.wantLabel {
				t.Fatalf("label() = %q, want %q", got, tt.wantLabel)
			}
		})
	}
}

func TestCellSyncPresentationQueuesExpectedUpdates(t *testing.T) {
	tests := []struct {
		name string
		view cellView
	}{
		{
			name: "revealed mine",
			view: cellView{revealed: true, mine: true},
		},
		{
			name: "flagged hidden",
			view: cellView{flagged: true},
		},
		{
			name: "hidden game over",
			view: cellView{gameOver: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newExampleRequest(t)
			g := newGame(2, 2, 1)
			elem := rq.NewElement(ui.NewButton(bind.MakeHTMLGetter("cell")))

			// A nil element is a no-op and must not emit anything.
			g.cells[0][0].syncPresentation(nil, tt.view)
			g.cells[0][0].syncPresentation(elem, tt.view)

			// Wake the harness process loop so it flushes the queued DOM ops to
			// OutCh: an empty (invalid-Jid) incoming message is ignored by
			// handleIncoming but still drives one loop iteration calling sendQueue.
			rq.InCh <- wire.WsMsg{}

			// The expected wire output mirrors syncPresentation: clear the four
			// state classes, set the classes for this view, set aria-label, and
			// set/clear the disabled attribute. Asserting the exact ops pins the
			// browser-visible behavior the cell presentation depends on.
			j := elem.Jid()
			var want []wire.WsMsg
			for _, cls := range []string{"is-hidden", "is-revealed", "is-flagged", "is-mine"} {
				want = append(want, wire.WsMsg{Data: cls, Jid: j, What: what.RClass})
			}
			if tt.view.revealed {
				want = append(want, wire.WsMsg{Data: "is-revealed", Jid: j, What: what.SClass})
				if tt.view.mine {
					want = append(want, wire.WsMsg{Data: "is-mine", Jid: j, What: what.SClass})
				}
			} else {
				want = append(want, wire.WsMsg{Data: "is-hidden", Jid: j, What: what.SClass})
				if tt.view.flagged {
					want = append(want, wire.WsMsg{Data: "is-flagged", Jid: j, What: what.SClass})
				}
			}
			want = append(want, wire.WsMsg{Data: "aria-label\n" + tt.view.label(), Jid: j, What: what.SAttr})
			if tt.view.revealed || tt.view.gameOver {
				want = append(want, wire.WsMsg{Data: "disabled\ndisabled", Jid: j, What: what.SAttr})
			} else {
				want = append(want, wire.WsMsg{Data: "disabled", Jid: j, What: what.RAttr})
			}

			got := make([]wire.WsMsg, 0, len(want))
			for i := 0; i < len(want); i++ {
				select {
				case msg := <-rq.OutCh:
					got = append(got, msg)
				case <-time.After(2 * time.Second):
					t.Fatalf("timed out after %d/%d messages; got %+v", i, len(want), got)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("queued updates mismatch:\n got %+v\nwant %+v", got, want)
			}
			// No further updates should be queued.
			select {
			case extra := <-rq.OutCh:
				t.Fatalf("unexpected extra update: %+v", extra)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestNewGameClampsAndExposesBoard(t *testing.T) {
	g := newGame(1, 1, 0)
	if g.rows != 2 || g.cols != 2 || g.mines != 1 {
		t.Fatalf("got rows=%d cols=%d mines=%d, want 2x2 with 1 mine", g.rows, g.cols, g.mines)
	}
	if len(g.Board()) != 2 || len(g.Board()[0]) != 2 {
		t.Fatalf("unexpected board dimensions: %#v", g.Board())
	}
	for row := range g.cells {
		for col, cell := range g.cells[row] {
			if cell.game != g || cell.row != row || cell.col != col {
				t.Fatalf("unexpected cell metadata at %d,%d: %#v", row, col, cell)
			}
			if cell.mine || cell.revealed || cell.flagged || cell.adjacent != 0 {
				t.Fatalf("expected reset cell at %d,%d, got %#v", row, col, cell)
			}
		}
	}

	g = newGame(2, 2, 10)
	if g.mines != 3 {
		t.Fatalf("mines = %d, want 3", g.mines)
	}

	g = newGameWithRand(2, 2, 1, nil)
	if g.rng == nil {
		t.Fatal("expected nil rng to be replaced")
	}
}

func TestGameStatusAndStatsHelpers(t *testing.T) {
	g := newGame(2, 3, 2)
	statusTests := []struct {
		name              string
		started, gameOver bool
		won               bool
		want              string
	}{
		{
			name: "initial",
			want: "Left-click reveals. Right-click toggles flags. First reveal is guaranteed safe.",
		},
		{
			name:    "started",
			started: true,
			want:    "Left-click reveals. Right-click toggles flags.",
		},
		{
			name:     "loss",
			started:  true,
			gameOver: true,
			want:     "Boom. You hit a mine.",
		},
		{
			name:     "win",
			started:  true,
			gameOver: true,
			won:      true,
			want:     "You cleared the board.",
		},
	}
	for _, tt := range statusTests {
		t.Run(tt.name, func(t *testing.T) {
			g.started = tt.started
			g.gameOver = tt.gameOver
			g.won = tt.won
			if got := g.statusText(); got != tt.want {
				t.Fatalf("statusText() = %q, want %q", got, tt.want)
			}
		})
	}

	statusAny := g.StatusSpan()
	statusGetter, ok := statusAny.(bind.Getter[string])
	if !ok {
		t.Fatalf("StatusSpan() type %T does not implement Getter[string]", statusAny)
	}
	statusTagger, ok := statusAny.(jawstag.TagGetter)
	if !ok {
		t.Fatalf("StatusSpan() type %T does not implement TagGetter", statusAny)
	}
	g.started = false
	g.gameOver = false
	g.won = false
	if got := statusGetter.JawsGet(nil); got != statusTests[0].want {
		t.Fatalf("StatusSpan getter = %q, want %q", got, statusTests[0].want)
	}
	statusTags, err := jawstag.TagExpand(nil, statusTagger.JawsGetTag(nil))
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, statusTags, &g.started, &g.gameOver, &g.won)

	g.revealed = 2
	g.flags = 1
	wantStats := "Mines: 2 | Flags: 1 | Safe cells left: 2"
	if got := g.statsText(); got != wantStats {
		t.Fatalf("statsText() = %q, want %q", got, wantStats)
	}
	statsAny := g.StatsSpan()
	statsGetter, ok := statsAny.(bind.Getter[string])
	if !ok {
		t.Fatalf("StatsSpan() type %T does not implement Getter[string]", statsAny)
	}
	statsTagger, ok := statsAny.(jawstag.TagGetter)
	if !ok {
		t.Fatalf("StatsSpan() type %T does not implement TagGetter", statsAny)
	}
	if got := statsGetter.JawsGet(nil); got != wantStats {
		t.Fatalf("StatsSpan getter = %q, want %q", got, wantStats)
	}
	statsTags, err := jawstag.TagExpand(nil, statsTagger.JawsGetTag(nil))
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, statsTags, &g.revealed, &g.flags)
}

func TestGameChangedTagsAndReset(t *testing.T) {
	fresh := newGame(2, 2, 1)
	if tags := fresh.reset(); tags != nil {
		t.Fatalf("fresh reset tags = %#v, want nil", tags)
	}

	g := newGame(2, 2, 1)
	before := g.snapshot()
	if tags := g.changedTags(before); tags != nil {
		t.Fatalf("changedTags() = %#v, want nil", tags)
	}

	g.started = true
	g.gameOver = true
	g.won = true
	g.revealed = 2
	g.flags = 1
	got := g.changedTags(before)
	assertTagSetEqual(t, got, &g.started, &g.gameOver, &g.won, &g.revealed, &g.flags)

	g.cells[0][0].mine = true
	g.cells[0][0].flagged = true
	g.cells[0][0].adjacent = 3
	tags := g.reset()
	assertTagSetEqual(t, tags, &g.started, &g.gameOver, &g.won, &g.revealed, &g.flags, &g.cells)
	if g.started || g.gameOver || g.won || g.revealed != 0 || g.flags != 0 {
		t.Fatalf("reset() left stale game state: %#v", g)
	}
	if g.cells[0][0].mine || g.cells[0][0].revealed || g.cells[0][0].flagged || g.cells[0][0].adjacent != 0 {
		t.Fatalf("reset() left stale cell state: %#v", g.cells[0][0])
	}
}

func TestNewGameButtonResetsBoard(t *testing.T) {
	_, rq := newExampleRequest(t)

	g := newGame(2, 2, 1)
	g.started = true
	g.gameOver = true
	g.won = true
	g.revealed = 1
	g.flags = 1
	g.cells[0][0].mine = true
	g.cells[0][0].flagged = true
	g.cells[0][0].adjacent = 3

	elem := rq.NewElement(ui.NewButton(g.NewGameButton()))
	var body bytes.Buffer
	if err := elem.JawsRender(&body, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.String(), "New game") {
		t.Fatalf("expected rendered button text, got %q", body.String())
	}
	if err := jaws.CallEventHandlers(elem.UI(), elem, what.Click, "0 0 0 new"); err != nil {
		t.Fatal(err)
	}
	if g.started || g.gameOver || g.won || g.revealed != 0 || g.flags != 0 {
		t.Fatalf("button click did not reset game: %#v", g)
	}
	if g.cells[0][0].mine || g.cells[0][0].flagged || g.cells[0][0].adjacent != 0 {
		t.Fatalf("button click did not reset cell: %#v", g.cells[0][0])
	}
}

func TestGameToggleFlagAndClickGuards(t *testing.T) {
	g := newGame(2, 2, 1)
	cell := g.cells[0][0]

	tags := g.toggleFlag(cell)
	if !cell.flagged || g.flags != 1 {
		t.Fatalf("toggleFlag() first toggle left flagged=%v flags=%d", cell.flagged, g.flags)
	}
	assertTagSetEqual(t, tags, cell, &g.flags)

	tags = g.toggleFlag(cell)
	if cell.flagged || g.flags != 0 {
		t.Fatalf("toggleFlag() second toggle left flagged=%v flags=%d", cell.flagged, g.flags)
	}
	assertTagSetEqual(t, tags, cell, &g.flags)

	g.gameOver = true
	if tags := g.toggleFlag(cell); tags != nil {
		t.Fatalf("toggleFlag() during game over = %#v, want nil", tags)
	}

	g.gameOver = false
	cell.revealed = true
	if tags := g.toggleFlag(cell); tags != nil {
		t.Fatalf("toggleFlag() on revealed cell = %#v, want nil", tags)
	}

	guardTests := []struct {
		name  string
		setup func(*game, *Cell)
	}{
		{
			name: "game over",
			setup: func(g *game, c *Cell) {
				g.gameOver = true
			},
		},
		{
			name: "flagged",
			setup: func(g *game, c *Cell) {
				c.flagged = true
			},
		},
		{
			name: "revealed",
			setup: func(g *game, c *Cell) {
				c.revealed = true
			},
		},
	}
	for _, tt := range guardTests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGame(2, 2, 1)
			cell := g.cells[0][0]
			tt.setup(g, cell)
			if tags := g.clickCell(cell); tags != nil {
				t.Fatalf("clickCell() = %#v, want nil", tags)
			}
		})
	}
}

func TestGameClickCellPaths(t *testing.T) {
	g := newGame(3, 3, 2)
	start := g.cells[1][1]
	tags := g.clickCell(start)
	if !g.started {
		t.Fatal("expected first click to start game")
	}
	if start.mine {
		t.Fatal("expected first click to stay safe")
	}
	if len(tags) == 0 {
		t.Fatal("expected dirty tags from first click")
	}

	g = newGame(2, 2, 1)
	g.started = true
	g.cells[0][0].mine = true
	g.calculateAdjacencyLocked()
	lossTags := g.clickCell(g.cells[0][0])
	if !g.gameOver || g.won {
		t.Fatalf("expected loss, got gameOver=%v won=%v", g.gameOver, g.won)
	}
	if !g.cells[0][0].revealed {
		t.Fatal("expected mine to be revealed on loss")
	}
	assertTagSetEqual(t, lossTags, &g.cells, &g.gameOver)

	g = newGame(2, 2, 1)
	g.started = true
	g.cells[1][1].mine = true
	g.calculateAdjacencyLocked()
	g.clickCell(g.cells[0][0])
	g.clickCell(g.cells[0][1])
	winTags := g.clickCell(g.cells[1][0])
	if !g.gameOver || !g.won {
		t.Fatalf("expected win, got gameOver=%v won=%v", g.gameOver, g.won)
	}
	if !g.cells[1][1].revealed {
		t.Fatal("expected remaining mine to be revealed on win")
	}
	assertTagSetEqual(t, winTags, &g.cells, &g.gameOver, &g.won, &g.revealed)
}

func containsTag(tags []any, want any) bool {
	for _, tg := range tags {
		if tg == want {
			return true
		}
	}
	return false
}

// TestSingleCellDirtyStaysScopedToOneCell guards the per-cell dirty contract after
// tag EXPANSION (what Request.Dirty actually does), not just the raw returned slice.
// A single-cell action (flag toggle) must not expand to the shared &g.cells board
// tag — otherwise every cell re-renders — while a board-wide refresh (reset) must.
// *Cell is a tag.TagGetter, so a board tag returned from Cell.JawsGetTag would be
// pulled in by expanding any single cell; this test fails if that regresses.
func TestSingleCellDirtyStaysScopedToOneCell(t *testing.T) {
	g := newGame(3, 3, 1)
	cell := g.cells[0][0]

	flagTags, err := jawstag.TagExpand(nil, g.toggleFlag(cell))
	if err != nil {
		t.Fatal(err)
	}
	// Expanded dirty set for a flag toggle is exactly the cell and the changed
	// flag counter — no board tag.
	assertTagSetEqual(t, flagTags, cell, &g.flags)

	// A board-wide refresh must still expand to the shared board tag so every cell
	// re-renders. Start the game first (the first click is guaranteed safe) so reset
	// reports changed scalars and appends &g.cells.
	g2 := newGame(3, 3, 1)
	_ = g2.clickCell(g2.cells[0][0])
	resetTags, err := jawstag.TagExpand(nil, g2.reset())
	if err != nil {
		t.Fatal(err)
	}
	if !containsTag(resetTags, &g2.cells) {
		t.Fatalf("board reset did not target the shared board tag &g.cells: %#v", resetTags)
	}
}

func TestRevealFromLockedAndRevealAllMines(t *testing.T) {
	g := newGame(3, 3, 2)
	g.cells[0][0].mine = true
	g.cells[2][2].mine = true
	g.calculateAdjacencyLocked()
	g.cells[0][1].flagged = true
	g.cells[2][1].revealed = true
	g.revealed = 1

	revealed := g.revealFromLocked(g.cells[2][0])
	if len(revealed) == 0 {
		t.Fatal("expected flood fill to reveal cells")
	}
	if g.cells[0][0].revealed {
		t.Fatal("expected mines to stay hidden during flood fill")
	}
	if g.cells[0][1].revealed {
		t.Fatal("expected flagged cells to stay hidden during flood fill")
	}
	if !g.cells[1][0].revealed {
		t.Fatal("expected neighboring safe cells to be revealed")
	}
	if g.revealed <= 1 {
		t.Fatalf("expected revealed count to increase, got %d", g.revealed)
	}

	g.revealAllMinesLocked()
	if !g.cells[0][0].revealed || !g.cells[2][2].revealed {
		t.Fatal("expected revealAllMinesLocked() to reveal all mines")
	}
}

func TestPlaceMinesLockedSkipsInitialCell(t *testing.T) {
	seed := findSeedWithSkipFirst(t, 2*2, 0)
	g := newGameWithRand(2, 2, 1, rand.New(rand.NewSource(seed)))
	skip := g.cells[0][0]

	g.placeMinesLocked(skip)

	if skip.mine {
		t.Fatal("expected skipped cell to stay safe")
	}
	mines := 0
	for row := range g.cells {
		for _, cell := range g.cells[row] {
			if cell.mine {
				mines++
			}
		}
	}
	if mines != g.mines {
		t.Fatalf("placed %d mines, want %d", mines, g.mines)
	}
}

func TestRunServesIndex(t *testing.T) {
	// Exercise the real production wiring of run end to end: inject only
	// listenAndServe so the test drives the assembled handler without binding a
	// port. The stub runs while the Jaws instance is still live (run has not
	// returned), issues a GET / through the fully wired handler, asserts the
	// rendered index page came back, then returns a sentinel run must propagate.
	want := errors.New("stop listening")
	var gotAddr string
	listen := func(addr string, handler http.Handler) error {
		gotAddr = addr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET / status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Minesweeper") {
			t.Errorf("GET / body missing page title, got %q", body)
		}
		// GenerateHeadHTML wiring: the jawsKey meta is always emitted, and our
		// extra stylesheet must be preloaded.
		if !strings.Contains(body, `name="jawsKey"`) {
			t.Errorf("GET / body missing JaWS head wiring, got %q", body)
		}
		if !strings.Contains(body, "/static/style.css") {
			t.Errorf("GET / body missing static stylesheet wiring, got %q", body)
		}
		return want
	}
	if err := run(listen); !errors.Is(err, want) {
		t.Fatalf("run() = %v, want %v", err, want)
	}
	if gotAddr != ":8080" {
		t.Errorf("listen addr = %q, want %q", gotAddr, ":8080")
	}
}
