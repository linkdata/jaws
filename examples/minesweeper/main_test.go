package main

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	jawstag "github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
)

func newExampleRequest(t *testing.T) (*jaws.Jaws, *jaws.TestRequest) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()

	rq := jaws.NewTestRequest(jw, httptest.NewRequest(http.MethodGet, "/", nil))
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

func tailScript(t *testing.T, jw *jaws.Jaws, rq *jaws.TestRequest) string {
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

func saveRunHooks(t *testing.T) {
	t.Helper()
	oldNewJaws := newJaws
	oldParseTemplates := parseMinesweeperTemplates
	oldAddTemplateLookuper := addTemplateLookuper
	oldGenerateHeadHTML := generateHeadHTML
	oldSubStaticFS := subStaticFS
	oldServeJaws := serveJaws
	oldListenAndServe := listenAndServe
	oldLogPrintln := logPrintln
	oldLogFatal := logFatal
	t.Cleanup(func() {
		newJaws = oldNewJaws
		parseMinesweeperTemplates = oldParseTemplates
		addTemplateLookuper = oldAddTemplateLookuper
		generateHeadHTML = oldGenerateHeadHTML
		subStaticFS = oldSubStaticFS
		serveJaws = oldServeJaws
		listenAndServe = oldListenAndServe
		logPrintln = oldLogPrintln
		logFatal = oldLogFatal
	})
}

func stubSuccessfulRunDeps(t *testing.T) {
	t.Helper()
	saveRunHooks(t)
	parseMinesweeperTemplates = func() (*template.Template, error) {
		return template.New("index.html"), nil
	}
	addTemplateLookuper = func(*jaws.Jaws, *template.Template) error { return nil }
	generateHeadHTML = func(*jaws.Jaws) error { return nil }
	subStaticFS = func() (fs.FS, error) { return fstest.MapFS{}, nil }
	serveJaws = func(*jaws.Jaws) {}
	listenAndServe = func(string, http.Handler) error { return nil }
	logPrintln = func(...any) {}
	logFatal = func(...any) {}
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

	var body bytes.Buffer
	if err := elem.JawsRender(&body, []any{`class="cell"`}); err != nil {
		t.Fatal(err)
	}
	if !elem.HasTag(cell) {
		t.Fatal("expected cell tag to be registered")
	}
	if !elem.HasTag(&g.cells) {
		t.Fatal("expected shared board tag to be registered")
	}

	if err := jaws.CallEventHandlers(elem.Ui(), elem, what.ContextMenu, "0 0 0 flag"); err != nil {
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
	if err := jaws.CallEventHandlers(otherElem.Ui(), otherElem, what.Click, "0 0 0 reveal"); err != nil {
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
			jw, rq := newExampleRequest(t)
			g := newGame(2, 2, 1)
			elem := rq.NewElement(ui.NewButton(bind.MakeHTMLGetter("cell")))

			g.cells[0][0].syncPresentation(nil, tt.view)
			g.cells[0][0].syncPresentation(elem, tt.view)

			_ = tailScript(t, jw, rq)
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
	if err := jaws.CallEventHandlers(elem.Ui(), elem, what.Click, "0 0 0 new"); err != nil {
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
	g := newGame(2, 2, 1)
	skip := g.cells[0][0]
	seed := findSeedWithSkipFirst(t, g.rows*g.cols, 0)
	g.rng = rand.New(rand.NewSource(seed))

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

func TestDefaultRunHooks(t *testing.T) {
	var logBuf bytes.Buffer
	oldWriter := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(oldWriter) })

	jw, err := newJaws()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	tmpl, err := parseMinesweeperTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Lookup("index.html") == nil {
		t.Fatal("expected embedded index.html template")
	}
	if err := addTemplateLookuper(jw, tmpl); err != nil {
		t.Fatal(err)
	}
	if err := generateHeadHTML(jw); err != nil {
		t.Fatal(err)
	}

	staticFiles, err := subStaticFS()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(staticFiles, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected embedded static files")
	}

	serveJaws(jw)
	logPrintln("hook smoke test")
	if !strings.Contains(logBuf.String(), "hook smoke test") {
		t.Fatalf("expected log output, got %q", logBuf.String())
	}
}

func TestDefaultLogFatal(t *testing.T) {
	if os.Getenv("JAWS_TEST_DEFAULT_LOG_FATAL") == "1" {
		logFatal(errors.New("boom"))
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestDefaultLogFatal")
	cmd.Env = append(os.Environ(), "JAWS_TEST_DEFAULT_LOG_FATAL=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected subprocess failure, got %v", err)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("expected fatal log output, got %q", stderr.String())
	}
}

func TestRunPropagatesDependencyErrors(t *testing.T) {
	tests := []struct {
		name  string
		patch func(error)
	}{
		{
			name: "new jaws",
			patch: func(want error) {
				newJaws = func() (*jaws.Jaws, error) { return nil, want }
			},
		},
		{
			name: "parse templates",
			patch: func(want error) {
				parseMinesweeperTemplates = func() (*template.Template, error) { return nil, want }
			},
		},
		{
			name: "add template lookuper",
			patch: func(want error) {
				addTemplateLookuper = func(*jaws.Jaws, *template.Template) error { return want }
			},
		},
		{
			name: "generate head html",
			patch: func(want error) {
				generateHeadHTML = func(*jaws.Jaws) error { return want }
			},
		},
		{
			name: "sub static fs",
			patch: func(want error) {
				subStaticFS = func() (fs.FS, error) { return nil, want }
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubSuccessfulRunDeps(t)
			want := errors.New(tt.name)
			serveJaws = func(*jaws.Jaws) { t.Fatal("serveJaws should not run on startup error") }
			listenAndServe = func(string, http.Handler) error {
				t.Fatal("listenAndServe should not run on startup error")
				return nil
			}
			tt.patch(want)
			if err := run(); !errors.Is(err, want) {
				t.Fatalf("run() error = %v, want %v", err, want)
			}
		})
	}
}

func TestRunBuildsServerAndReturnsListenError(t *testing.T) {
	stubSuccessfulRunDeps(t)
	want := errors.New("listen")
	var served bool
	var printed bool
	var gotAddr string
	var gotHandler http.Handler

	serveJaws = func(*jaws.Jaws) { served = true }
	logPrintln = func(...any) { printed = true }
	listenAndServe = func(addr string, handler http.Handler) error {
		gotAddr = addr
		gotHandler = handler
		return want
	}

	if err := run(); !errors.Is(err, want) {
		t.Fatalf("run() error = %v, want %v", err, want)
	}
	if !served {
		t.Fatal("expected serveJaws to run")
	}
	if !printed {
		t.Fatal("expected startup log line")
	}
	if gotAddr != ":8080" {
		t.Fatalf("listen address = %q, want %q", gotAddr, ":8080")
	}
	if gotHandler == nil {
		t.Fatal("expected a mux handler")
	}
}

func TestMainFatalBehavior(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stubSuccessfulRunDeps(t)
		fatalCalled := false
		logFatal = func(...any) { fatalCalled = true }
		listenAndServe = func(string, http.Handler) error { return nil }

		main()

		if fatalCalled {
			t.Fatal("logFatal should not run when run() succeeds")
		}
	})

	t.Run("error", func(t *testing.T) {
		stubSuccessfulRunDeps(t)
		want := errors.New("listen")
		var got error
		logFatal = func(v ...any) {
			if len(v) != 1 {
				t.Fatalf("logFatal args = %#v, want single error", v)
			}
			var ok bool
			got, ok = v[0].(error)
			if !ok {
				t.Fatalf("logFatal arg type = %T, want error", v[0])
			}
		}
		listenAndServe = func(string, http.Handler) error { return want }

		main()

		if !errors.Is(got, want) {
			t.Fatalf("logFatal error = %v, want %v", got, want)
		}
	})
}
