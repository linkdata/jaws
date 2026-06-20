package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/ui"
)

//go:embed assets/ui/*.html assets/static/*.css
var assetsFS embed.FS

// Cell represents one Minesweeper board cell.
type Cell struct {
	game     *game
	row, col int
	mine     bool
	revealed bool
	flagged  bool
	adjacent int
}

// cellView is an immutable snapshot of a Cell's display state, taken under the
// game lock so rendering can run without holding it.
type cellView struct {
	mine     bool
	revealed bool
	flagged  bool
	gameOver bool
	adjacent int
}

func newCell(g *game, row, col int) *Cell {
	return &Cell{game: g, row: row, col: col}
}

// reset clears the cell to its initial hidden state.
func (c *Cell) reset() {
	c.mine = false
	c.revealed = false
	c.flagged = false
	c.adjacent = 0
}

// toggleFlag toggles the flagged state and returns the new state.
func (c *Cell) toggleFlag() bool {
	c.flagged = !c.flagged
	return c.flagged
}

func (c *Cell) snapshotLocked() cellView {
	return cellView{
		mine:     c.mine,
		revealed: c.revealed,
		flagged:  c.flagged,
		gameOver: c.game.gameOver,
		adjacent: c.adjacent,
	}
}

func (v cellView) HTML() template.HTML {
	if v.revealed {
		if v.mine {
			return template.HTML(`<span class="glyph glyph-mine">☠</span>`) // #nosec G203
		}
		if v.adjacent > 0 {
			return template.HTML(`<span class="cleared">` + strconv.Itoa(v.adjacent) + `</span>`) // #nosec G203
		}
		return template.HTML(`<span class="cleared"></span>`) // #nosec G203
	}
	if v.flagged {
		return template.HTML(`<span class="glyph glyph-flag">⚑</span>`) // #nosec G203
	}
	return ""
}

func (v cellView) label() string {
	switch {
	case v.revealed && v.mine:
		return "Mine"
	case v.revealed && v.adjacent > 0:
		return fmt.Sprintf("Revealed cell with %d adjacent mines", v.adjacent)
	case v.revealed:
		return "Revealed empty cell"
	case v.flagged:
		return "Flagged hidden cell"
	case v.gameOver:
		return "Hidden cell, game over"
	default:
		return "Hidden cell"
	}
}

func (c *Cell) syncPresentation(elem *jaws.Element, view cellView) {
	if elem == nil {
		return
	}
	for _, cls := range []string{"is-hidden", "is-revealed", "is-flagged", "is-mine"} {
		elem.RemoveClass(cls)
	}
	if view.revealed {
		elem.SetClass("is-revealed")
		if view.mine {
			elem.SetClass("is-mine")
		}
	} else {
		elem.SetClass("is-hidden")
		if view.flagged {
			elem.SetClass("is-flagged")
		}
	}
	elem.SetAttr("aria-label", view.label())
	if view.revealed || view.gameOver {
		elem.SetAttr("disabled", "disabled")
	} else {
		elem.RemoveAttr("disabled")
	}
}

// JawsGetTag returns the cell's per-cell dirty identity.
//
// It deliberately returns ONLY the cell, not the shared board tag. Because *Cell
// is a [tag.TagGetter], returning the board tag here would make Request.Dirty(c)
// tag-expand to include it, so dirtying one cell would re-render every cell. The
// shared board tag is registered separately via [Cell.BoardTag] (passed to the
// cell's Button), so the element listens to both while a single-cell dirty stays
// scoped to just that cell. Board-wide refreshes dirty &g.cells directly.
func (c *Cell) JawsGetTag(_ tag.Context) any {
	return c
}

// BoardTag returns the shared board dirty tag registered on every cell element.
//
// Board-wide refreshes (reset, loss, win) dirty this tag to re-render the whole
// board at once. It is kept separate from [Cell.JawsGetTag] so per-cell dirtying
// stays scoped; see that method for why.
func (c *Cell) BoardTag() any {
	return &c.game.cells
}

// JawsGetHTML renders the cell contents.
func (c *Cell) JawsGetHTML(elem *jaws.Element) template.HTML {
	c.game.mu.Lock()
	view := c.snapshotLocked()
	c.game.mu.Unlock()
	c.syncPresentation(elem, view)
	return view.HTML()
}

// JawsClick reveals the cell.
func (c *Cell) JawsClick(elem *jaws.Element, click jaws.Click) error {
	elem.Request.Dirty(c.game.clickCell(c)...)
	return nil
}

// JawsContextMenu toggles the cell flag.
func (c *Cell) JawsContextMenu(elem *jaws.Element, click jaws.Click) error {
	elem.Request.Dirty(c.game.toggleFlag(c)...)
	return nil
}

type game struct {
	mu sync.Mutex

	rows  int
	cols  int
	mines int

	cells    [][]*Cell
	rng      *rand.Rand
	started  bool
	gameOver bool
	won      bool
	revealed int
	flags    int
}

func newGame(rows, cols, mines int) *game {
	return newGameWithRand(rows, cols, mines, nil)
}

func newGameWithRand(rows, cols, mines int, rng *rand.Rand) *game {
	if rows < 2 {
		rows = 2
	}
	if cols < 2 {
		cols = 2
	}
	total := rows * cols
	if mines >= total {
		mines = total - 1
	}
	if mines < 1 {
		mines = 1
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404
	}

	g := &game{
		rows:  rows,
		cols:  cols,
		mines: mines,
		rng:   rng,
	}
	g.cells = make([][]*Cell, rows)
	for row := 0; row < rows; row++ {
		g.cells[row] = make([]*Cell, cols)
		for col := 0; col < cols; col++ {
			g.cells[row][col] = newCell(g, row, col)
		}
	}
	g.resetLocked()
	return g
}

// gameState is a snapshot of the scalar fields that UI getters depend on.
// Mutations snapshot-before and diff-after so they only dirty scalar deps
// whose values actually changed — JaWS does not diff HTMLInner updates, so
// a spurious dirty would re-render the status/stats lines on every click.
type gameState struct {
	started, gameOver, won bool
	revealed, flags        int
}

func (g *game) snapshot() gameState {
	return gameState{
		started:  g.started,
		gameOver: g.gameOver,
		won:      g.won,
		revealed: g.revealed,
		flags:    g.flags,
	}
}

// changedTags returns the tags whose bound game state differs from the before
// snapshot, so only the elements that actually changed are marked dirty.
func (g *game) changedTags(before gameState) (tags []any) {
	if g.started != before.started {
		tags = append(tags, &g.started)
	}
	if g.gameOver != before.gameOver {
		tags = append(tags, &g.gameOver)
	}
	if g.won != before.won {
		tags = append(tags, &g.won)
	}
	if g.revealed != before.revealed {
		tags = append(tags, &g.revealed)
	}
	if g.flags != before.flags {
		tags = append(tags, &g.flags)
	}
	return
}

// Board returns the board cells for template iteration.
//
// The grid shape is fixed after newGame; only per-cell fields mutate, always
// under g.mu, and each cell is re-read under the same lock while rendering, so
// concurrent template iteration is safe.
func (g *game) Board() [][]*Cell { return g.cells }

// StatusSpan returns a dynamic status text getter.
func (g *game) StatusSpan() any {
	return bind.StringGetterFunc(func(*jaws.Element) string {
		return g.statusText()
	}, &g.started, &g.gameOver, &g.won)
}

// StatsSpan returns a dynamic statistics text getter.
func (g *game) StatsSpan() any {
	return bind.StringGetterFunc(func(*jaws.Element) string {
		return g.statsText()
	}, &g.revealed, &g.flags)
}

// NewGameButton returns a button object that starts a new game.
func (g *game) NewGameButton() ui.Object {
	return ui.New("New game").Clicked(func(_ ui.Object, elem *jaws.Element, _ jaws.Click) error {
		elem.Request.Dirty(g.reset()...)
		return nil
	})
}

func (g *game) reset() []any {
	g.mu.Lock()
	defer g.mu.Unlock()
	before := g.snapshot()
	g.resetLocked()
	tags := g.changedTags(before)
	if len(tags) > 0 {
		tags = append(tags, &g.cells)
	}
	return tags
}

func (g *game) resetLocked() {
	for row := 0; row < g.rows; row++ {
		for col := 0; col < g.cols; col++ {
			g.cells[row][col].reset()
		}
	}
	g.started = false
	g.gameOver = false
	g.won = false
	g.revealed = 0
	g.flags = 0
}

func (g *game) statusText() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case g.gameOver && g.won:
		return "You cleared the board."
	case g.gameOver:
		return "Boom. You hit a mine."
	case !g.started:
		return "Left-click reveals. Right-click toggles flags. First reveal is guaranteed safe."
	default:
		return "Left-click reveals. Right-click toggles flags."
	}
}

func (g *game) statsText() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	remaining := g.rows*g.cols - g.mines - g.revealed
	return fmt.Sprintf("Mines: %d | Flags: %d | Safe cells left: %d", g.mines, g.flags, remaining)
}

func (g *game) clickCell(cell *Cell) []any {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.gameOver || cell.flagged || cell.revealed {
		return nil
	}
	before := g.snapshot()

	if !g.started {
		g.placeMinesLocked(cell)
		g.started = true
	}

	var cellTags []any
	if cell.mine {
		g.gameOver = true
		g.revealAllMinesLocked()
		cellTags = []any{&g.cells} // all mines revealed; refresh board
	} else {
		revealed := g.revealFromLocked(cell)
		if g.revealed == g.rows*g.cols-g.mines {
			g.gameOver = true
			g.won = true
			g.revealAllMinesLocked()
			cellTags = []any{&g.cells} // win reveals remaining mines
		} else {
			// []*Cell does not assign to []any, so copy element-wise.
			for _, c := range revealed {
				cellTags = append(cellTags, c)
			}
		}
	}
	return append(cellTags, g.changedTags(before)...)
}

func (g *game) toggleFlag(cell *Cell) []any {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gameOver || cell.revealed {
		return nil
	}
	before := g.snapshot()
	if cell.toggleFlag() {
		g.flags++
	} else {
		g.flags--
	}
	return append([]any{cell}, g.changedTags(before)...)
}

func (g *game) revealFromLocked(start *Cell) (revealed []*Cell) {
	stack := []*Cell{start}
	for len(stack) > 0 {
		last := len(stack) - 1
		cell := stack[last]
		stack = stack[:last]

		if cell.revealed || cell.flagged || cell.mine {
			continue
		}
		cell.revealed = true
		revealed = append(revealed, cell)
		g.revealed++
		if cell.adjacent != 0 {
			continue
		}
		g.forEachNeighborLocked(cell, func(neighbor *Cell) {
			stack = append(stack, neighbor)
		})
	}
	return
}

func (g *game) revealAllMinesLocked() {
	for row := 0; row < g.rows; row++ {
		for col := 0; col < g.cols; col++ {
			cell := g.cells[row][col]
			if cell.mine {
				cell.revealed = true
			}
		}
	}
}

func (g *game) forEachNeighborLocked(cell *Cell, fn func(*Cell)) {
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			row := cell.row + dr
			col := cell.col + dc
			if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
				continue
			}
			fn(g.cells[row][col])
		}
	}
}

func (g *game) placeMinesLocked(skip *Cell) {
	skipIdx := skip.row*g.cols + skip.col
	placed := 0
	for _, idx := range g.rng.Perm(g.rows * g.cols) {
		if placed >= g.mines {
			break
		}
		if idx == skipIdx {
			continue
		}
		g.cells[idx/g.cols][idx%g.cols].mine = true
		placed++
	}
	g.calculateAdjacencyLocked()
}

func (g *game) calculateAdjacencyLocked() {
	for row := 0; row < g.rows; row++ {
		for col := 0; col < g.cols; col++ {
			cell := g.cells[row][col]
			if cell.mine {
				continue
			}
			count := 0
			g.forEachNeighborLocked(cell, func(neighbor *Cell) {
				if neighbor.mine {
					count++
				}
			})
			cell.adjacent = count
		}
	}
}

// run wires up the JaWS Minesweeper demo and serves it.
//
// listenAndServe is the only injected dependency: production passes
// [http.ListenAndServe], while tests pass a stub so they can drive the wired
// handler without binding a real port. Everything else is constructed inline so
// the function reads as a copyable "how to wire up JaWS" template.
func run(listenAndServe func(addr string, handler http.Handler) error) error {
	jw, err := jaws.New()
	if err == nil {
		defer jw.Close()
		var tmpl *template.Template
		if tmpl, err = template.ParseFS(assetsFS, "assets/ui/*.html"); err == nil {
			if err = jw.AddTemplateLookuper(tmpl); err == nil {
				if err = jw.GenerateHeadHTML("/static/style.css"); err == nil {
					var staticFiles fs.FS
					if staticFiles, err = fs.Sub(assetsFS, "assets/static"); err == nil {
						// One board is shared by every visitor: this is a single, collaborative
						// game that all connected browsers see and play simultaneously. That is a
						// deliberate choice for this demo. For per-user state instead, create the
						// game inside the handler (or in a JawsInit) keyed off the request/session,
						// e.g. via jw.Session, rather than binding one board for the whole server.
						board := newGame(10, 10, 15)

						mux := http.NewServeMux()
						mux.Handle("GET /jaws/", jw)
						mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
						mux.Handle("GET /", jw.SessionMiddleware(jw.SecureHeadersMiddleware(ui.Handler(jw, "index.html", board))))

						go jw.Serve()
						log.Println("Minesweeper listening on http://localhost:8080")
						err = listenAndServe(":8080", mux)
					}
				}
			}
		}
	}
	return err
}

func main() {
	if err := run(http.ListenAndServe); err != nil {
		log.Fatal(err)
	}
}
