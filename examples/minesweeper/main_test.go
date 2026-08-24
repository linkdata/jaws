package main

import (
	"bytes"
	"errors"
	"html/template"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstest"
	jawstag "github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

func newExampleJaws(t *testing.T) *jaws.Jaws {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	go jw.Serve()
	t.Cleanup(jw.Close)
	return jw
}

func newRequestFor(t *testing.T, jw *jaws.Jaws) *jawstest.TestRequest {
	t.Helper()
	rq := jawstest.NewTestRequest(jw, httptest.NewRequest(http.MethodGet, "/", nil))
	<-rq.ReadyCh
	t.Cleanup(func() {
		rq.Close()
		for {
			select {
			case <-rq.OutCh:
			case <-rq.DoneCh:
				return
			}
		}
	})
	return rq
}

func newExampleRequest(t *testing.T) (*jaws.Jaws, *jawstest.TestRequest) {
	t.Helper()
	jw := newExampleJaws(t)
	rq := newRequestFor(t, jw)
	return jw, rq
}

func drainWire(t *testing.T, rq *jawstest.TestRequest, probe string) (messages []wire.WsMsg) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case rq.InCh <- wire.WsMsg{}:
	case <-rq.DoneCh:
		t.Fatal("request stopped before accepting the wire wakeup")
	case <-timer.C:
		t.Fatal("timed out sending the wire wakeup")
	}
	select {
	case rq.BcastCh <- wire.Message{What: what.Alert, Data: probe}:
	case <-rq.DoneCh:
		t.Fatal("request stopped before accepting the wire probe")
	case <-timer.C:
		t.Fatal("timed out sending the wire probe")
	}
	for {
		select {
		case msg := <-rq.OutCh:
			if msg.What == what.Alert && msg.Data == probe {
				return
			}
			messages = append(messages, msg)
		case <-rq.DoneCh:
			t.Fatal("request stopped before the wire probe arrived")
		case <-timer.C:
			t.Fatal("timed out waiting for the wire probe")
		}
	}
}

func assertTagSetEqual(t *testing.T, got []any, want ...any) {
	t.Helper()
	gotSet := make(map[any]struct{}, len(got))
	for i, v := range got {
		if _, exists := gotSet[v]; exists {
			t.Fatalf("got duplicate tag at index %d: %#v", i, v)
		}
		gotSet[v] = struct{}{}
	}
	wantSet := make(map[any]struct{}, len(want))
	for i, v := range want {
		if _, exists := wantSet[v]; exists {
			t.Fatalf("want duplicate tag at index %d: %#v", i, v)
		}
		wantSet[v] = struct{}{}
	}
	if !reflect.DeepEqual(gotSet, wantSet) {
		t.Fatalf("tag set mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func cellButtonParams(c *cell) []any {
	return []any{c.BoardTag(), c.GameOverTag(), template.HTMLAttr(`class="cell"`)}
}

type blockingInitialCell struct {
	*cell
	read   chan struct{}
	resume chan struct{}
}

func (source *blockingInitialCell) JawsInitialHTMLAttr(elem *jaws.Element) template.HTMLAttr {
	attrs := source.cell.JawsInitialHTMLAttr(elem)
	close(source.read)
	<-source.resume
	return attrs
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
	_, rq := newExampleRequest(t)

	g := newGame(3, 3, 1)
	cell := g.cells[0][0]
	elem := rq.NewElement(ui.NewButton(cell))

	var body bytes.Buffer
	if err := elem.JawsRender(&body, cellButtonParams(cell)); err != nil {
		t.Fatal(err)
	}
	if !elem.HasTag(cell) {
		t.Fatal("expected cell tag to be registered")
	}
	if !elem.HasTag(&g.cells) {
		t.Fatal("expected shared board tag to be registered")
	}
	if !elem.HasTag(&g.gameOver) {
		t.Fatal("expected game-over tag to be registered")
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
	if err := jaws.CallEventHandlers(elem.UI(), elem, what.Click, "0 0 1 flag"); err != nil {
		t.Fatalf("Shift-click error: %v", err)
	}
	if cell.flagged || g.flags != 0 {
		t.Fatalf("Shift-click left flagged=%v flags=%d, want false and 0", cell.flagged, g.flags)
	}

	other := g.cells[0][1]
	otherElem := rq.NewElement(ui.NewButton(other))
	if err := otherElem.JawsRender(&body, cellButtonParams(other)); err != nil {
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
}

func TestIndexTemplateRegistersPreciseDependencies(t *testing.T) {
	_, rq := newExampleRequest(t)
	tmpl, err := template.ParseFS(assetsFS, "assets/ui/*.html")
	if err != nil {
		t.Fatal(err)
	}

	g := newGame(3, 3, 1)
	var body strings.Builder
	rw := ui.RequestWriter{Request: rq.Request, Writer: &body}
	if err = tmpl.ExecuteTemplate(&body, "index.html", ui.With{RequestWriter: rw, Dot: g}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(body.String(), "<!doctype html>") {
		t.Fatalf("page does not start with the doctype: %q", body.String())
	}
	if got := len(rq.GetElements(&g.cells)); got != 9 {
		t.Fatalf("board tag has %d cell Elements, want 9", got)
	}
	for _, row := range g.cells {
		for _, cell := range row {
			if got := len(rq.GetElements(cell)); got != 1 {
				t.Fatalf("cell %d,%d tag has %d Elements, want 1", cell.row, cell.col, got)
			}
		}
	}
	for name, dependency := range map[string]struct {
		tag   any
		count int
	}{
		"started":   {tag: &g.started, count: 1},
		"game over": {tag: &g.gameOver, count: 10},
		"won":       {tag: &g.won, count: 1},
		"revealed":  {tag: &g.revealed, count: 1},
		"flags":     {tag: &g.flags, count: 1},
	} {
		if got := len(rq.GetElements(dependency.tag)); got != dependency.count {
			t.Fatalf("%s tag has %d Elements, want %d", name, got, dependency.count)
		}
	}
}

func TestCellButtonRendersAndUpdatesAuthoritativeState(t *testing.T) {
	tests := []struct {
		name         string
		configure    func(*game, *cell)
		wantHTML     string
		wantLabel    string
		wantState    string
		wantDisabled bool
	}{
		{
			name: "revealed mine",
			configure: func(_ *game, cell *cell) {
				cell.revealed = true
				cell.mine = true
			},
			wantHTML:     `<span class="glyph glyph-mine">☠</span>`,
			wantLabel:    "Row 1, column 1: mine",
			wantState:    "mine",
			wantDisabled: true,
		},
		{
			name: "revealed adjacent",
			configure: func(_ *game, cell *cell) {
				cell.revealed = true
				cell.adjacent = 3
			},
			wantHTML:     `<span class="cleared">3</span>`,
			wantLabel:    "Row 1, column 1: revealed with 3 adjacent mines",
			wantState:    "revealed",
			wantDisabled: true,
		},
		{
			name: "revealed empty",
			configure: func(_ *game, cell *cell) {
				cell.revealed = true
			},
			wantHTML:     `<span class="cleared"></span>`,
			wantLabel:    "Row 1, column 1: revealed with no adjacent mines",
			wantState:    "revealed",
			wantDisabled: true,
		},
		{
			name: "flagged hidden",
			configure: func(_ *game, cell *cell) {
				cell.flagged = true
			},
			wantHTML:  `<span class="glyph glyph-flag">⚑</span>`,
			wantLabel: "Row 1, column 1: flagged",
			wantState: "flagged",
		},
		{
			name: "hidden game over",
			configure: func(g *game, _ *cell) {
				g.gameOver = true
			},
			wantLabel:    "Row 1, column 1: hidden; game over",
			wantState:    "hidden",
			wantDisabled: true,
		},
		{
			name:      "hidden",
			configure: func(*game, *cell) {},
			wantLabel: "Row 1, column 1: hidden",
			wantState: "hidden",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, rq := newExampleRequest(t)
			g := newGame(2, 2, 1)
			cell := g.cells[0][0]
			tt.configure(g, cell)

			elem := rq.NewElement(ui.NewButton(cell))
			var body strings.Builder
			if err := elem.JawsRender(&body, cellButtonParams(cell)); err != nil {
				t.Fatal(err)
			}
			for _, fragment := range []string{
				`class="cell"`,
				`data-state="` + tt.wantState + `"`,
				`aria-label="` + tt.wantLabel + `"`,
			} {
				if !strings.Contains(body.String(), fragment) {
					t.Errorf("initial Button %q does not contain %q", body.String(), fragment)
				}
			}
			if tt.wantHTML != "" && !strings.Contains(body.String(), tt.wantHTML) {
				t.Errorf("initial Button %q does not contain %q", body.String(), tt.wantHTML)
			}
			if got := strings.Contains(body.String(), `disabled="disabled"`); got != tt.wantDisabled {
				t.Errorf("initial Button disabled = %v, want %v", got, tt.wantDisabled)
			}

			j := elem.Jid()
			wantAttrs := []wire.WsMsg{
				{Data: "data-state\n" + tt.wantState, Jid: j, What: what.SAttr},
				{Data: "aria-label\n" + tt.wantLabel, Jid: j, What: what.SAttr},
			}
			if tt.wantDisabled {
				wantAttrs = append(wantAttrs, wire.WsMsg{Data: "disabled\ndisabled", Jid: j, What: what.SAttr})
			} else {
				wantAttrs = append(wantAttrs, wire.WsMsg{Data: "disabled", Jid: j, What: what.RAttr})
			}
			if got := drainWire(t, rq, "initial "+tt.name); !reflect.DeepEqual(got, wantAttrs) {
				t.Fatalf("initial Button fixups mismatch:\n got %+v\nwant %+v", got, wantAttrs)
			}
			elem.JawsUpdate()
			want := append([]wire.WsMsg(nil), wantAttrs...)
			want = append(want, wire.WsMsg{Data: tt.wantHTML, Jid: j, What: what.Inner})

			got := drainWire(t, rq, "update "+tt.name)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Button update mismatch:\n got %+v\nwant %+v", got, want)
			}
		})
	}
}

func TestCellInitialReadsConvergeThroughTailAttributes(t *testing.T) {
	_, rq := newExampleRequest(t)
	g := newGame(2, 2, 1)
	cell := g.cells[0][0]
	source := &blockingInitialCell{
		cell:   cell,
		read:   make(chan struct{}),
		resume: make(chan struct{}),
	}
	elem := rq.NewElement(ui.NewButton(source))

	var body strings.Builder
	rendered := make(chan error, 1)
	go func() {
		rendered <- elem.JawsRender(&body, cellButtonParams(cell))
	}()

	select {
	case <-source.read:
	case <-t.Context().Done():
		close(source.resume)
		t.Fatal(t.Context().Err())
	}
	tags := g.toggleFlag(cell)
	close(source.resume)
	if len(tags) == 0 {
		t.Fatal("toggleFlag() returned no dirty tags")
	}
	select {
	case err := <-rendered:
		if err != nil {
			t.Fatal(err)
		}
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}

	for _, fragment := range []string{
		`data-state="hidden"`,
		`aria-label="Row 1, column 1: hidden"`,
		string(flagHTML),
	} {
		if !strings.Contains(body.String(), fragment) {
			t.Errorf("initial Button %q does not contain %q", body.String(), fragment)
		}
	}

	// The mutation lands after the initial attributes but before ApplyParams
	// registers the shared tags. The standard Button's getter still queues the
	// current wrapper state for TailHTML without retaining a presentation model.
	want := []wire.WsMsg{
		{Data: "data-state\nflagged", Jid: elem.Jid(), What: what.SAttr},
		{Data: "aria-label\nRow 1, column 1: flagged", Jid: elem.Jid(), What: what.SAttr},
		{Data: "disabled", Jid: elem.Jid(), What: what.RAttr},
	}
	if got := drainWire(t, rq, "initial read convergence"); !reflect.DeepEqual(got, want) {
		t.Fatalf("initial Button fixups mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestCellButtonUpdatePreservesTemplateClasses(t *testing.T) {
	_, rq := newExampleRequest(t)
	g := newGame(2, 2, 1)
	cell := g.cells[0][0]
	elem := rq.NewElement(ui.NewButton(cell))
	params := []any{cell.BoardTag(), cell.GameOverTag(), template.HTMLAttr(`class="cell custom"`)}
	var body strings.Builder
	if err := elem.JawsRender(&body, params); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.String(), `class="cell custom"`) {
		t.Fatalf("initial Button did not retain template classes: %q", body.String())
	}
	wantInitial := []wire.WsMsg{
		{Data: "data-state\nhidden", Jid: elem.Jid(), What: what.SAttr},
		{Data: "aria-label\nRow 1, column 1: hidden", Jid: elem.Jid(), What: what.SAttr},
		{Data: "disabled", Jid: elem.Jid(), What: what.RAttr},
	}
	if got := drainWire(t, rq, "initial custom class"); !reflect.DeepEqual(got, wantInitial) {
		t.Fatalf("initial Button fixups mismatch:\n got %+v\nwant %+v", got, wantInitial)
	}

	if tags := g.toggleFlag(cell); len(tags) == 0 {
		t.Fatal("toggleFlag() returned no dirty tags")
	}
	elem.JawsUpdate()

	j := elem.Jid()
	want := []wire.WsMsg{
		{Data: "data-state\nflagged", Jid: j, What: what.SAttr},
		{Data: "aria-label\nRow 1, column 1: flagged", Jid: j, What: what.SAttr},
		{Data: "disabled", Jid: j, What: what.RAttr},
		{Data: string(flagHTML), Jid: j, What: what.Inner},
	}
	if got := drainWire(t, rq, "custom class update"); !reflect.DeepEqual(got, want) {
		t.Fatalf("Button update mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestConnectedRequestsConvergeAfterDirtying(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw := newExampleJaws(t)
		first := newRequestFor(t, jw)
		second := newRequestFor(t, jw)
		g := newGame(3, 3, 1)
		target := g.cells[0][0]
		other := g.cells[0][1]

		type rendered struct {
			request                   *jawstest.TestRequest
			target, other, statistics *jaws.Element
		}
		clients := make([]rendered, 0, 2)
		for _, rq := range []*jawstest.TestRequest{first, second} {
			var body strings.Builder
			targetElem := rq.NewElement(ui.NewButton(target))
			if err := targetElem.JawsRender(&body, cellButtonParams(target)); err != nil {
				t.Fatal(err)
			}
			otherElem := rq.NewElement(ui.NewButton(other))
			if err := otherElem.JawsRender(&body, cellButtonParams(other)); err != nil {
				t.Fatal(err)
			}
			statsElem := rq.NewElement(ui.NewSpan(g.Stats()))
			if err := statsElem.JawsRender(&body, nil); err != nil {
				t.Fatal(err)
			}
			clients = append(clients, rendered{
				request:    rq,
				target:     targetElem,
				other:      otherElem,
				statistics: statsElem,
			})
			_ = drainWire(t, rq, "initial collaborative render")
		}

		resetElem := first.NewElement(ui.NewButton(g.NewGameAction()))
		if err := resetElem.JawsRender(&strings.Builder{}, nil); err != nil {
			t.Fatal(err)
		}
		waitForUpdates := func() {
			synctest.Wait()
			time.Sleep(jaws.DefaultUpdateInterval + time.Millisecond)
			synctest.Wait()
		}

		first.InCh <- wire.WsMsg{Jid: clients[0].target.Jid(), What: what.ContextMenu, Data: "0 0 0 flag"}
		waitForUpdates()
		if !target.flagged || g.flags != 1 {
			t.Fatalf("context menu left flagged=%v flags=%d, want true and 1", target.flagged, g.flags)
		}
		for _, client := range clients {
			messages := drainWire(t, client.request, "flag updates")
			assertOnlyElementUpdates(t, messages, client.target, client.statistics)
			assertCellUpdates(t, messages, client.target, "flagged", "Row 1, column 1: flagged", flagHTML, false)
			assertInnerUpdate(t, messages, client.statistics, "Mines: 1 | Flags: 1 | Safe cells left: 8")
		}

		first.InCh <- wire.WsMsg{Jid: resetElem.Jid(), What: what.Click, Data: "0 0 0 reset"}
		waitForUpdates()
		if target.flagged || g.flags != 0 {
			t.Fatalf("reset left flagged=%v flags=%d, want false and 0", target.flagged, g.flags)
		}
		for _, client := range clients {
			messages := drainWire(t, client.request, "reset updates")
			assertOnlyElementUpdates(t, messages, client.target, client.other, client.statistics)
			assertCellUpdates(t, messages, client.target, "hidden", "Row 1, column 1: hidden", "", false)
			assertCellUpdates(t, messages, client.other, "hidden", "Row 1, column 2: hidden", "", false)
			assertInnerUpdate(t, messages, client.statistics, "Mines: 1 | Flags: 0 | Safe cells left: 8")
		}

		// Independent Request loops may mutate the shared game concurrently. The
		// game lock serializes those writes, while JaWS coalesces their dirty tags.
		first.InCh <- wire.WsMsg{Jid: clients[0].target.Jid(), What: what.ContextMenu, Data: "0 0 0 flag target"}
		second.InCh <- wire.WsMsg{Jid: clients[1].other.Jid(), What: what.ContextMenu, Data: "0 0 0 flag other"}
		waitForUpdates()
		if !target.flagged || !other.flagged || g.flags != 2 {
			t.Fatalf("concurrent flags left target=%v other=%v flags=%d, want true, true, 2", target.flagged, other.flagged, g.flags)
		}
		for _, client := range clients {
			messages := drainWire(t, client.request, "concurrent updates")
			assertOnlyElementUpdates(t, messages, client.target, client.other, client.statistics)
			assertCellUpdates(t, messages, client.target, "flagged", "Row 1, column 1: flagged", flagHTML, false)
			assertCellUpdates(t, messages, client.other, "flagged", "Row 1, column 2: flagged", flagHTML, false)
			assertInnerUpdate(t, messages, client.statistics, "Mines: 1 | Flags: 2 | Safe cells left: 8")
		}

		g.mu.Lock()
		g.gameOver = true
		g.mu.Unlock()
		first.Dirty(&g.gameOver)
		waitForUpdates()
		for _, client := range clients {
			messages := drainWire(t, client.request, "game-over updates")
			assertOnlyElementUpdates(t, messages, client.target, client.other)
			assertCellUpdates(t, messages, client.target, "flagged", "Row 1, column 1: flagged", flagHTML, true)
			assertCellUpdates(t, messages, client.other, "flagged", "Row 1, column 2: flagged", flagHTML, true)
		}
	})
}

func assertOnlyElementUpdates(t *testing.T, messages []wire.WsMsg, want ...*jaws.Element) {
	t.Helper()
	seen := make(map[jaws.Jid]bool, len(want))
	for _, elem := range want {
		seen[elem.Jid()] = false
	}
	for _, msg := range messages {
		if _, ok := seen[msg.Jid]; !ok {
			t.Fatalf("unexpected update for %v: %+v", msg.Jid, msg)
		}
		seen[msg.Jid] = true
	}
	for jid, found := range seen {
		if !found {
			t.Fatalf("expected an update for %v, got %+v", jid, messages)
		}
	}
}

func assertCellUpdates(t *testing.T, messages []wire.WsMsg, elem *jaws.Element, state, label string, inner template.HTML, disabled bool) {
	t.Helper()
	disabledUpdate := wire.WsMsg{Data: "disabled", What: what.RAttr}
	if disabled {
		disabledUpdate = wire.WsMsg{Data: "disabled\ndisabled", What: what.SAttr}
	}
	assertElementMessages(
		t, messages, elem,
		wire.WsMsg{Data: "data-state\n" + state, What: what.SAttr},
		wire.WsMsg{Data: "aria-label\n" + label, What: what.SAttr},
		disabledUpdate,
		wire.WsMsg{Data: string(inner), What: what.Inner},
	)
}

func assertInnerUpdate(t *testing.T, messages []wire.WsMsg, elem *jaws.Element, inner string) {
	t.Helper()
	assertElementMessages(t, messages, elem, wire.WsMsg{Data: inner, What: what.Inner})
}

func assertElementMessages(t *testing.T, messages []wire.WsMsg, elem *jaws.Element, want ...wire.WsMsg) {
	t.Helper()
	got := make([]wire.WsMsg, 0, len(want))
	for _, msg := range messages {
		if msg.Jid == elem.Jid() {
			got = append(got, msg)
		}
	}
	for i := range want {
		want[i].Jid = elem.Jid()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("updates for %v mismatch:\n got %+v\nwant %+v", elem.Jid(), got, want)
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
			want: "Click reveals. Right-click or Shift-click toggles flags. The first reveal is safe.",
		},
		{
			name:    "started",
			started: true,
			want:    "Click reveals. Right-click or Shift-click toggles flags.",
		},
		{
			name:     "loss",
			started:  true,
			gameOver: true,
			want:     "A mine was revealed.",
		},
		{
			name:     "win",
			started:  true,
			gameOver: true,
			won:      true,
			want:     "The shared board is clear.",
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

	statusGetter := g.Status()
	statusTagger, ok := statusGetter.(jawstag.TagGetter)
	if !ok {
		t.Fatalf("Status() type %T does not implement TagGetter", statusGetter)
	}
	g.started = false
	g.gameOver = false
	g.won = false
	if got := statusGetter.JawsGet(nil); got != statusTests[0].want {
		t.Fatalf("Status getter = %q, want %q", got, statusTests[0].want)
	}
	statusTags, err := jawstag.TagExpand(statusTagger.JawsGetTag())
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
	statsGetter := g.Stats()
	statsTagger, ok := statsGetter.(jawstag.TagGetter)
	if !ok {
		t.Fatalf("Stats() type %T does not implement TagGetter", statsGetter)
	}
	if got := statsGetter.JawsGet(nil); got != wantStats {
		t.Fatalf("Stats getter = %q, want %q", got, wantStats)
	}
	statsTags, err := jawstag.TagExpand(statsTagger.JawsGetTag())
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, statsTags, &g.revealed, &g.flags)
}

func TestGameResetReturnsOnlyChangedTags(t *testing.T) {
	fresh := newGame(2, 2, 1)
	if tags := fresh.reset(); tags != nil {
		t.Fatalf("fresh reset tags = %#v, want nil", tags)
	}

	flagged := newGame(2, 2, 1)
	flaggedCell := flagged.cells[0][0]
	_ = flagged.toggleFlag(flaggedCell)
	tags := flagged.reset()
	assertTagSetEqual(t, tags, &flagged.flags, &flagged.cells)
	if flaggedCell.flagged || flagged.flags != 0 {
		t.Fatalf("reset() left a pre-start flag: cell=%#v flags=%d", flaggedCell, flagged.flags)
	}

	g := newGame(2, 2, 1)
	g.started = true
	g.gameOver = true
	g.won = true
	g.revealed = 2
	g.flags = 1
	g.cells[0][0].mine = true
	g.cells[0][0].flagged = true
	g.cells[0][0].adjacent = 3
	tags = g.reset()
	assertTagSetEqual(t, tags, &g.started, &g.gameOver, &g.won, &g.revealed, &g.flags, &g.cells)
	if g.started || g.gameOver || g.won || g.revealed != 0 || g.flags != 0 {
		t.Fatalf("reset() left stale game state: %#v", g)
	}
	if g.cells[0][0].mine || g.cells[0][0].revealed || g.cells[0][0].flagged || g.cells[0][0].adjacent != 0 {
		t.Fatalf("reset() left stale cell state: %#v", g.cells[0][0])
	}
}

func TestNewGameActionResetsBoard(t *testing.T) {
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

	elem := rq.NewElement(ui.NewButton(g.NewGameAction()))
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
	target := g.cells[0][0]

	tags := g.toggleFlag(target)
	if !target.flagged || g.flags != 1 {
		t.Fatalf("toggleFlag() first toggle left flagged=%v flags=%d", target.flagged, g.flags)
	}
	assertTagSetEqual(t, tags, target, &g.flags)

	tags = g.toggleFlag(target)
	if target.flagged || g.flags != 0 {
		t.Fatalf("toggleFlag() second toggle left flagged=%v flags=%d", target.flagged, g.flags)
	}
	assertTagSetEqual(t, tags, target, &g.flags)

	g.gameOver = true
	if tags := g.toggleFlag(target); tags != nil {
		t.Fatalf("toggleFlag() during game over = %#v, want nil", tags)
	}
	if !g.gameOver || target.flagged || g.flags != 0 {
		t.Fatalf("toggleFlag() mutated a finished game: gameOver=%v flagged=%v flags=%d", g.gameOver, target.flagged, g.flags)
	}

	g.gameOver = false
	target.revealed = true
	if tags := g.toggleFlag(target); tags != nil {
		t.Fatalf("toggleFlag() on revealed cell = %#v, want nil", tags)
	}
	if g.gameOver || !target.revealed || target.flagged || g.flags != 0 {
		t.Fatalf("toggleFlag() mutated a revealed cell: gameOver=%v revealed=%v flagged=%v flags=%d", g.gameOver, target.revealed, target.flagged, g.flags)
	}

	guardTests := []struct {
		name             string
		setup            func(*game, *cell)
		wantGameOver     bool
		wantRevealed     int
		wantFlags        int
		wantCellRevealed bool
		wantCellFlagged  bool
	}{
		{
			name: "game over",
			setup: func(g *game, c *cell) {
				g.gameOver = true
			},
			wantGameOver: true,
		},
		{
			name: "flagged",
			setup: func(g *game, c *cell) {
				c.flagged = true
				g.flags = 1
			},
			wantFlags:       1,
			wantCellFlagged: true,
		},
		{
			name: "revealed",
			setup: func(g *game, c *cell) {
				c.revealed = true
				g.revealed = 1
			},
			wantRevealed:     1,
			wantCellRevealed: true,
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
			if g.started || g.gameOver != tt.wantGameOver || g.won || g.revealed != tt.wantRevealed || g.flags != tt.wantFlags {
				t.Fatalf("clickCell() mutated guarded game state: %#v", g)
			}
			if cell.revealed != tt.wantCellRevealed || cell.flagged != tt.wantCellFlagged || cell.mine || cell.adjacent != 0 {
				t.Fatalf("clickCell() mutated guarded cell state: %#v", cell)
			}
			for row := range g.cells {
				for col, current := range g.cells[row] {
					if current.mine || current.adjacent != 0 {
						t.Fatalf("clickCell() placed a mine during guarded reveal at %d,%d: %#v", row, col, current)
					}
				}
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
	assertTagSetEqual(t, tags, start, &g.started, &g.revealed)

	g = newGame(2, 2, 3)
	start = g.cells[0][0]
	immediateWinTags := g.clickCell(start)
	if !g.started || !g.gameOver || !g.won || !start.revealed || start.mine {
		t.Fatalf("immediate win left unexpected state: game=%#v start=%#v", g, start)
	}
	for row := range g.cells {
		for col, cell := range g.cells[row] {
			if cell != start && (!cell.mine || !cell.revealed) {
				t.Fatalf("immediate win left mine %d,%d hidden or absent: %#v", row, col, cell)
			}
		}
	}
	assertTagSetEqual(t, immediateWinTags, &g.started, &g.cells, &g.gameOver, &g.won, &g.revealed)

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

func TestCalculateAdjacencyLocked(t *testing.T) {
	g := newGame(3, 3, 2)
	g.cells[0][0].mine = true
	g.cells[2][2].mine = true
	g.calculateAdjacencyLocked()

	want := [][]int{
		{-1, 1, 0},
		{1, 2, 1},
		{0, 1, -1},
	}
	for row := range g.cells {
		for col, cell := range g.cells[row] {
			if cell.mine {
				if want[row][col] != -1 {
					t.Fatalf("cell %d,%d is an unexpected mine", row, col)
				}
				continue
			}
			if cell.adjacent != want[row][col] {
				t.Fatalf("cell %d,%d adjacency = %d, want %d", row, col, cell.adjacent, want[row][col])
			}
		}
	}
}

func containsTag(tags []any, want any) bool {
	for _, tg := range tags {
		if tg == want {
			return true
		}
	}
	return false
}

// TestSingleCellDirtyStaysScopedToOneCell guards narrow and board-wide dirtying.
func TestSingleCellDirtyStaysScopedToOneCell(t *testing.T) {
	g := newGame(3, 3, 1)
	cell := g.cells[0][0]

	flagTags, err := jawstag.TagExpand(g.toggleFlag(cell))
	if err != nil {
		t.Fatal(err)
	}
	assertTagSetEqual(t, flagTags, cell, &g.flags)

	g2 := newGame(3, 3, 1)
	_ = g2.clickCell(g2.cells[0][0])
	resetTags, err := jawstag.TagExpand(g2.reset())
	if err != nil {
		t.Fatal(err)
	}
	if !containsTag(resetTags, &g2.cells) {
		t.Fatalf("board reset did not target the shared board tag &g.cells: %#v", resetTags)
	}
}

func TestRevealFromLockedAndRevealAllMines(t *testing.T) {
	g := newGame(4, 4, 1)
	g.cells[0][0].mine = true
	g.calculateAdjacencyLocked()
	g.cells[2][2].flagged = true

	revealed := g.revealFromLocked(g.cells[3][3])
	if len(revealed) != 14 || g.revealed != 14 {
		t.Fatalf("flood fill revealed %d cells with count %d, want 14", len(revealed), g.revealed)
	}
	for row := range g.cells {
		for col, cell := range g.cells[row] {
			wantRevealed := !(row == 0 && col == 0) && !(row == 2 && col == 2)
			if cell.revealed != wantRevealed {
				t.Errorf("cell %d,%d revealed = %v, want %v", row, col, cell.revealed, wantRevealed)
			}
		}
	}

	g.revealAllMinesLocked()
	if !g.cells[0][0].revealed {
		t.Fatal("expected revealAllMinesLocked() to reveal all mines")
	}
}

func TestClickCellReturnsEveryFloodFillTag(t *testing.T) {
	g := newGame(4, 4, 1)
	g.started = true
	g.cells[0][0].mine = true
	g.cells[2][2].flagged = true
	g.flags = 1
	g.calculateAdjacencyLocked()

	tags := g.clickCell(g.cells[3][3])
	want := []any{&g.revealed}
	for row := range g.cells {
		for col, cell := range g.cells[row] {
			if row == 0 && col == 0 || row == 2 && col == 2 {
				continue
			}
			want = append(want, cell)
		}
	}
	assertTagSetEqual(t, tags, want...)
	if g.gameOver || g.won || g.revealed != 14 {
		t.Fatalf("flood-fill click left gameOver=%v won=%v revealed=%d, want false, false, 14", g.gameOver, g.won, g.revealed)
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

func TestRunServesApplication(t *testing.T) {
	want := errors.New("stop listening")
	var gotAddr string
	listen := func(addr string, handler http.Handler) error {
		gotAddr = addr
		serve := func(path string) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			return rec
		}

		page := serve("/")
		if page.Code != http.StatusOK {
			t.Errorf("GET / status = %d, want %d", page.Code, http.StatusOK)
		}
		body := page.Body.String()
		for _, fragment := range []string{
			"<!doctype html>",
			"Minesweeper",
			`name="jawsKey"`,
			"/static/style.css",
			`role="region" aria-label="Scrollable Minesweeper board" tabindex="0"`,
			`class="cell" data-state="hidden"`,
			`aria-label="Row 1, column 1: hidden"`,
		} {
			if !strings.Contains(body, fragment) {
				t.Errorf("GET / body does not contain %q", fragment)
			}
		}
		if !strings.HasPrefix(body, "<!doctype html>") {
			t.Errorf("GET / body does not start with a doctype: %q", body)
		}
		if got := strings.Count(body, `role="status"`); got != 2 {
			t.Errorf("GET / has %d status regions, want 2", got)
		}
		if strings.Contains(body, `aria-atomic="true"`) {
			t.Error("GET / wraps independent status updates in one atomic live region")
		}
		if got := page.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET / Cache-Control = %q, want no-store", got)
		}
		if got := page.Header().Get("Content-Security-Policy"); got == "" {
			t.Error("GET / is missing Content-Security-Policy")
		}
		if got := page.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("GET / X-Content-Type-Options = %q, want nosniff", got)
		}
		if cookies := page.Header().Values("Set-Cookie"); len(cookies) != 0 {
			t.Errorf("GET / set unused session cookies: %q", cookies)
		}

		if path, ok := tailEndpoint(body); !ok {
			t.Error("GET / body is missing the TailHTML endpoint")
		} else {
			tail := serve(path)
			if tail.Code != http.StatusOK {
				t.Errorf("initial TailHTML status = %d, want %d", tail.Code, http.StatusOK)
			}
			tailScript := tail.Body.String()
			for operation, want := range map[string]int{
				`setAttribute("data-state","hidden")`: 100,
				`setAttribute("aria-label",`:          100,
				`removeAttribute("disabled")`:         100,
			} {
				if got := strings.Count(tailScript, operation); got != want {
					t.Errorf("initial TailHTML contains %d %s operations, want %d", got, operation, want)
				}
			}
			if strings.Contains(tailScript, `setAttribute("class",`) {
				t.Error("initial TailHTML replaces the template-owned class")
			}
		}

		stylesheet := serve("/static/style.css")
		if stylesheet.Code != http.StatusOK {
			t.Errorf("GET /static/style.css status = %d, want %d", stylesheet.Code, http.StatusOK)
		}
		if !strings.Contains(stylesheet.Body.String(), ".cell") {
			t.Error("GET /static/style.css body is missing cell styles")
		}

		ping := serve("/jaws/.ping")
		if ping.Code != http.StatusNoContent {
			t.Errorf("GET /jaws/.ping status = %d, want %d", ping.Code, http.StatusNoContent)
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
