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
	"github.com/linkdata/jaws/lib/ui"
)

//go:embed assets/ui/*.html assets/static/*.css
var assetsFS embed.FS

type Cell struct {
	row int
	col int

	mine     bool
	revealed bool
	flagged  bool
	adjacent int
}

func newCell(row, col int) *Cell {
	return &Cell{row: row, col: col}
}

func (c *Cell) Reset() {
	c.mine = false
	c.revealed = false
	c.flagged = false
	c.adjacent = 0
}

func (c *Cell) ToggleFlag() bool {
	c.flagged = !c.flagged
	return c.flagged
}

func (c *Cell) Text() string {
	if c.revealed {
		if c.mine {
			return "☠"
		}
		if c.adjacent > 0 {
			return strconv.Itoa(c.adjacent)
		}
		return ""
	}
	if c.flagged {
		return "⚑"
	}
	return ""
}

func (c *Cell) HTML() template.HTML {
	if c.revealed {
		if c.mine {
			return template.HTML(`<span class="glyph glyph-mine">☠</span>`) // #nosec G203
		}
		if c.adjacent > 0 {
			return template.HTML(strconv.Itoa(c.adjacent)) // #nosec G203
		}
		return ""
	}
	if c.flagged {
		return template.HTML(`<span class="glyph glyph-flag">⚑</span>`) // #nosec G203
	}
	return ""
}

type dirtySet struct {
	cells []*Cell
	tags  []any
}

func (d dirtySet) apply(rq *jaws.Request) {
	if rq == nil {
		return
	}
	tagCount := len(d.cells) + len(d.tags)
	if tagCount == 0 {
		return
	}

	seen := make(map[any]struct{}, tagCount)
	tags := make([]any, 0, tagCount)
	for _, cell := range d.cells {
		if cell == nil {
			continue
		}
		if _, exists := seen[cell]; exists {
			continue
		}
		seen[cell] = struct{}{}
		tags = append(tags, cell)
	}
	for _, tag := range d.tags {
		if tag == nil {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	if len(tags) > 0 {
		rq.Dirty(tags...)
	}
}

type game struct {
	mu sync.Mutex

	rows  int
	cols  int
	mines int

	rowIndexes []int
	colIndexes []int

	cells    [][]*Cell
	rng      *rand.Rand
	started  bool
	gameOver bool
	won      bool
	revealed int
	flags    int
}

func newGame(rows, cols, mines int) *game {
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

	g := &game{
		rows:       rows,
		cols:       cols,
		mines:      mines,
		rowIndexes: make([]int, rows),
		colIndexes: make([]int, cols),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for i := range g.rowIndexes {
		g.rowIndexes[i] = i
	}
	for i := range g.colIndexes {
		g.colIndexes[i] = i
	}
	g.initCellsLocked()
	g.resetLocked()
	return g
}

func (g *game) initCellsLocked() {
	g.cells = make([][]*Cell, g.rows)
	for row := 0; row < g.rows; row++ {
		g.cells[row] = make([]*Cell, g.cols)
		for col := 0; col < g.cols; col++ {
			g.cells[row][col] = newCell(row, col)
		}
	}
}

func (g *game) statusDeps() []any {
	return []any{&g.started, &g.gameOver, &g.won}
}

func (g *game) boardDep() any {
	return &g.cells
}

func (g *game) statsDeps() []any {
	return []any{&g.revealed, &g.flags}
}

func (g *game) allStateDeps() []any {
	deps := make([]any, 0, len(g.statusDeps())+len(g.statsDeps()))
	deps = append(deps, g.statusDeps()...)
	deps = append(deps, g.statsDeps()...)
	return deps
}

func (g *game) Rows() []int {
	return g.rowIndexes
}

func (g *game) Cols() []int {
	return g.colIndexes
}

func (g *game) StatusSpan() any {
	return bind.StringGetterFunc(func(*jaws.Element) string {
		return g.statusText()
	}, g.statusDeps()...)
}

func (g *game) StatsSpan() any {
	return bind.StringGetterFunc(func(*jaws.Element) string {
		return g.statsText()
	}, g.statsDeps()...)
}

func (g *game) NewGameButton() ui.Object {
	return ui.New("New game").Clicked(func(_ ui.Object, elem *jaws.Element, _ jaws.Click) error {
		g.reset()
		tags := g.allStateDeps()
		tags = append(tags, g.boardDep())
		dirtySet{
			tags: tags,
		}.apply(elem.Request)
		return nil
	})
}

func (g *game) CellButton(row, col int) ui.Object {
	cell := g.cellAt(row, col)
	if cell == nil {
		return ui.New("?")
	}
	return ui.New(bind.HTMLGetterFunc(func(*jaws.Element) template.HTML {
		return g.cellHTML(cell)
	}, cell, g.boardDep())).Clicked(func(_ ui.Object, elem *jaws.Element, _ jaws.Click) error {
		g.clickCell(cell).apply(elem.Request)
		return nil
	}).ContextMenu(func(_ ui.Object, elem *jaws.Element, _ jaws.Click) error {
		g.rightClickCell(cell).apply(elem.Request)
		return nil
	})
}

func (g *game) cellAt(row, col int) *Cell {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cellAtLocked(row, col)
}

func (g *game) cellAtLocked(row, col int) *Cell {
	if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
		return nil
	}
	return g.cells[row][col]
}

func (g *game) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.resetLocked()
}

func (g *game) resetLocked() {
	for row := 0; row < g.rows; row++ {
		for col := 0; col < g.cols; col++ {
			g.cells[row][col].Reset()
		}
	}
	g.started = false
	g.gameOver = false
	g.won = false
	g.revealed = 0
	g.flags = 0
}

func (g *game) safeCellCount() int {
	return g.rows*g.cols - g.mines
}

func (g *game) statusText() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.statusTextLocked()
}

func (g *game) statusTextLocked() string {
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
	remaining := g.safeCellCount() - g.revealed
	return fmt.Sprintf("Mines: %d | Flags: %d | Safe cells left: %d", g.mines, g.flags, remaining)
}

func (g *game) cellHTML(cell *Cell) template.HTML {
	g.mu.Lock()
	defer g.mu.Unlock()
	if cell == nil {
		return "?"
	}
	return cell.HTML()
}

func (g *game) appendChangedStateDeps(
	dirty *dirtySet,
	startedBefore, gameOverBefore, wonBefore bool,
	revealedBefore, flagsBefore int,
) {
	if dirty == nil {
		return
	}
	if g.started != startedBefore {
		dirty.tags = append(dirty.tags, &g.started)
	}
	if g.gameOver != gameOverBefore {
		dirty.tags = append(dirty.tags, &g.gameOver)
	}
	if g.won != wonBefore {
		dirty.tags = append(dirty.tags, &g.won)
	}
	if g.revealed != revealedBefore {
		dirty.tags = append(dirty.tags, &g.revealed)
	}
	if g.flags != flagsBefore {
		dirty.tags = append(dirty.tags, &g.flags)
	}
}

func (g *game) maybeUseBoardDirty(dirty *dirtySet) {
	if dirty == nil {
		return
	}
	// Keep one Dirty() call safely below JaWS tag expansion limits by switching
	// to the shared board dependency tag for broad refreshes.
	if len(dirty.cells)+len(dirty.tags) > 95 {
		dirty.cells = nil
		dirty.tags = append(dirty.tags, g.boardDep())
	}
}

func (g *game) clickCell(cell *Cell) (dirty dirtySet) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if cell == nil || g.gameOver {
		return dirty
	}

	startedBefore := g.started
	gameOverBefore := g.gameOver
	wonBefore := g.won
	revealedBefore := g.revealed
	flagsBefore := g.flags

	if cell.flagged || cell.revealed {
		return dirty
	}

	if !g.started {
		g.placeMinesLocked(cell)
		g.started = true
	}

	if cell.mine {
		g.gameOver = true
		g.won = false
		dirty.cells = append(dirty.cells, g.revealAllMinesLocked()...)
		g.appendChangedStateDeps(
			&dirty,
			startedBefore, gameOverBefore, wonBefore,
			revealedBefore, flagsBefore,
		)
		g.maybeUseBoardDirty(&dirty)
		return dirty
	}

	dirty.cells = append(dirty.cells, g.revealFromLocked(cell)...)
	if g.revealed == g.safeCellCount() {
		g.gameOver = true
		g.won = true
		dirty.cells = append(dirty.cells, g.revealAllMinesLocked()...)
	}

	g.appendChangedStateDeps(
		&dirty,
		startedBefore, gameOverBefore, wonBefore,
		revealedBefore, flagsBefore,
	)
	g.maybeUseBoardDirty(&dirty)
	return dirty
}

func (g *game) rightClickCell(cell *Cell) (dirty dirtySet) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if cell == nil || g.gameOver || cell.revealed {
		return dirty
	}

	startedBefore := g.started
	gameOverBefore := g.gameOver
	wonBefore := g.won
	revealedBefore := g.revealed
	flagsBefore := g.flags

	if cell.ToggleFlag() {
		g.flags++
	} else {
		g.flags--
	}
	dirty.cells = append(dirty.cells, cell)
	g.appendChangedStateDeps(
		&dirty,
		startedBefore, gameOverBefore, wonBefore,
		revealedBefore, flagsBefore,
	)
	return dirty
}

func (g *game) revealFromLocked(start *Cell) (revealed []*Cell) {
	if start == nil {
		return revealed
	}
	stack := []*Cell{start}
	for len(stack) > 0 {
		last := len(stack) - 1
		cell := stack[last]
		stack = stack[:last]

		if cell == nil {
			continue
		}
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
	return revealed
}

func (g *game) revealAllMinesLocked() (revealed []*Cell) {
	for row := 0; row < g.rows; row++ {
		for col := 0; col < g.cols; col++ {
			cell := g.cells[row][col]
			if cell.mine && !cell.revealed {
				cell.revealed = true
				revealed = append(revealed, cell)
			}
		}
	}
	return revealed
}

func (g *game) forEachNeighborLocked(cell *Cell, fn func(*Cell)) {
	if cell == nil || fn == nil {
		return
	}
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
	choices := make([]*Cell, 0, g.rows*g.cols-1)
	for row := 0; row < g.rows; row++ {
		for col := 0; col < g.cols; col++ {
			cell := g.cells[row][col]
			if cell == skip {
				continue
			}
			choices = append(choices, cell)
		}
	}

	g.rng.Shuffle(len(choices), func(i, j int) {
		choices[i], choices[j] = choices[j], choices[i]
	})

	mineCount := g.mines
	if mineCount > len(choices) {
		mineCount = len(choices)
	}
	for i := 0; i < mineCount; i++ {
		choices[i].mine = true
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

func main() {
	jw, err := jaws.New()
	if err != nil {
		log.Fatal(err)
	}
	defer jw.Close()

	tmpl, err := template.ParseFS(assetsFS, "assets/ui/*.html")
	if err != nil {
		log.Fatal(err)
	}
	if err = jw.AddTemplateLookuper(tmpl); err != nil {
		log.Fatal(err)
	}
	if err = jw.GenerateHeadHTML("/static/style.css"); err != nil {
		log.Fatal(err)
	}

	staticFiles, err := fs.Sub(assetsFS, "assets/static")
	if err != nil {
		log.Fatal(err)
	}

	board := newGame(10, 10, 15)

	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
	mux.Handle("GET /", jw.Session(jw.SecureHeadersMiddleware(ui.Handler(jw, "index.html", board))))

	go jw.Serve()
	log.Println("Minesweeper listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
