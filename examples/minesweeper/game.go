package main

import (
	"math/rand"
	"sync"
	"time"
)

type cell struct {
	game     *game
	row, col int
	mine     bool
	revealed bool
	flagged  bool
	adjacent int
}

func (c *cell) reset() {
	c.mine = false
	c.revealed = false
	c.flagged = false
	c.adjacent = 0
}

type game struct {
	mu sync.Mutex

	rows  int
	cols  int
	mines int

	cells    [][]*cell
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
		rng = rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 -- mine placement is not security-sensitive
	}

	g := &game{
		rows:  rows,
		cols:  cols,
		mines: mines,
		rng:   rng,
		cells: make([][]*cell, rows),
	}
	for row := range g.cells {
		g.cells[row] = make([]*cell, cols)
		for col := range g.cells[row] {
			g.cells[row][col] = &cell{game: g, row: row, col: col}
		}
	}
	g.resetLocked()
	return g
}

// reset clears the game and returns the changed dependency tags.
func (g *game) reset() (tags []any) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// A started game has placed mines. Before the first reveal, a nonzero flag
	// count is the only way any cell can differ from its reset state.
	boardChanged := g.started || g.flags != 0
	if g.started {
		tags = append(tags, &g.started)
	}
	if g.gameOver {
		tags = append(tags, &g.gameOver)
	}
	if g.won {
		tags = append(tags, &g.won)
	}
	if g.revealed != 0 {
		tags = append(tags, &g.revealed)
	}
	if g.flags != 0 {
		tags = append(tags, &g.flags)
	}
	g.resetLocked()
	if boardChanged {
		tags = append(tags, &g.cells)
	}
	return
}

func (g *game) resetLocked() {
	for _, row := range g.cells {
		for _, current := range row {
			current.reset()
		}
	}
	g.started = false
	g.gameOver = false
	g.won = false
	g.revealed = 0
	g.flags = 0
}

// clickCell applies a reveal attempt and returns the changed dependency tags.
func (g *game) clickCell(target *cell) (tags []any) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.gameOver || target.flagged || target.revealed {
		return
	}
	if !g.started {
		g.placeMinesLocked(target)
		g.started = true
		tags = append(tags, &g.started)
	}

	if target.mine {
		g.gameOver = true
		g.revealAllMinesLocked()
		tags = append(tags, &g.cells, &g.gameOver)
		return
	}

	revealed := g.revealFromLocked(target)
	if g.revealed == g.rows*g.cols-g.mines {
		g.gameOver = true
		g.won = true
		g.revealAllMinesLocked()
		tags = append(tags, &g.cells, &g.gameOver, &g.won, &g.revealed)
		return
	}
	for _, revealedCell := range revealed {
		tags = append(tags, revealedCell)
	}
	tags = append(tags, &g.revealed)
	return
}

// toggleFlag applies a flag attempt and returns the changed dependency tags.
func (g *game) toggleFlag(target *cell) (tags []any) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.gameOver || target.revealed {
		return
	}
	target.flagged = !target.flagged
	if target.flagged {
		g.flags++
	} else {
		g.flags--
	}
	tags = []any{target, &g.flags}
	return
}

func (g *game) revealFromLocked(start *cell) (revealed []*cell) {
	stack := []*cell{start}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]

		if current.revealed || current.flagged || current.mine {
			continue
		}
		current.revealed = true
		revealed = append(revealed, current)
		g.revealed++
		if current.adjacent != 0 {
			continue
		}
		g.forEachNeighborLocked(current, func(neighbor *cell) {
			stack = append(stack, neighbor)
		})
	}
	return
}

func (g *game) revealAllMinesLocked() {
	for _, row := range g.cells {
		for _, current := range row {
			if current.mine {
				current.revealed = true
			}
		}
	}
}

func (g *game) forEachNeighborLocked(center *cell, fn func(*cell)) {
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			row := center.row + dr
			col := center.col + dc
			if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
				continue
			}
			fn(g.cells[row][col])
		}
	}
}

func (g *game) placeMinesLocked(skip *cell) {
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
	for _, row := range g.cells {
		for _, current := range row {
			if current.mine {
				continue
			}
			count := 0
			g.forEachNeighborLocked(current, func(neighbor *cell) {
				if neighbor.mine {
					count++
				}
			})
			current.adjacent = count
		}
	}
}
