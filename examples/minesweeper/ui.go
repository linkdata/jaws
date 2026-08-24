package main

import (
	"fmt"
	"html/template"
	"strconv"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/ui"
)

const (
	mineHTML  template.HTML = `<span class="glyph glyph-mine">☠</span>`
	flagHTML  template.HTML = `<span class="glyph glyph-flag">⚑</span>`
	emptyHTML template.HTML = `<span class="cleared"></span>`
)

func (c *cell) htmlLocked() template.HTML {
	if c.revealed {
		if c.mine {
			return mineHTML
		}
		if c.adjacent > 0 {
			return template.HTML(`<span class="cleared">` + strconv.Itoa(c.adjacent) + `</span>`) // #nosec G203 -- fixed markup contains only an integer
		}
		return emptyHTML
	}
	if c.flagged {
		return flagHTML
	}
	return ""
}

func (c *cell) labelLocked() string {
	prefix := fmt.Sprintf("Row %d, column %d: ", c.row+1, c.col+1)
	switch {
	case c.revealed && c.mine:
		return prefix + "mine"
	case c.revealed && c.adjacent > 0:
		return fmt.Sprintf("%srevealed with %d adjacent mines", prefix, c.adjacent)
	case c.revealed:
		return prefix + "revealed with no adjacent mines"
	case c.flagged:
		return prefix + "flagged"
	case c.game.gameOver:
		return prefix + "hidden; game over"
	default:
		return prefix + "hidden"
	}
}

func (c *cell) classLocked() string {
	switch {
	case c.revealed && c.mine:
		return "cell is-revealed is-mine"
	case c.revealed:
		return "cell is-revealed"
	case c.flagged:
		return "cell is-hidden is-flagged"
	default:
		return "cell is-hidden"
	}
}

func (c *cell) initialAttrsLocked() (attrs template.HTMLAttr) {
	attrs = htmlio.Attr("class", c.classLocked())
	attrs += " " + htmlio.Attr("aria-label", c.labelLocked())
	if c.revealed || c.game.gameOver {
		attrs += " " + htmlio.Attr("disabled", "disabled")
	}
	return
}

func (c *cell) updateAttributesLocked(elem *jaws.Element) {
	elem.SetAttr("class", c.classLocked())
	elem.SetAttr("aria-label", c.labelLocked())
	if c.revealed || c.game.gameOver {
		elem.SetAttr("disabled", "disabled")
	} else {
		elem.RemoveAttr("disabled")
	}
}

// JawsGetTag returns the precise dependency tag for this cell.
func (c *cell) JawsGetTag() any {
	// Register shared dependencies separately so Dirty(c) does not expand into
	// a board-wide refresh.
	return c
}

// BoardTag returns the shared dependency for board-wide refreshes.
func (c *cell) BoardTag() any {
	return &c.game.cells
}

// GameOverTag returns the shared dependency for terminal presentation changes.
func (c *cell) GameOverTag() any {
	return &c.game.gameOver
}

// JawsInitialHTMLAttr returns the cell's initial presentation attributes.
func (c *cell) JawsInitialHTMLAttr(_ *jaws.Element) template.HTMLAttr {
	c.game.mu.Lock()
	defer c.game.mu.Unlock()

	return c.initialAttrsLocked()
}

// JawsGetHTML returns the cell's current markup and queues its Button attributes.
func (c *cell) JawsGetHTML(elem *jaws.Element) template.HTML {
	c.game.mu.Lock()
	defer c.game.mu.Unlock()

	// Dirtying the cell schedules every matching Element. Each batched update
	// reads the shared state here, so connected browsers converge on that state.
	c.updateAttributesLocked(elem)
	return c.htmlLocked()
}

// JawsClick handles a reveal or Shift-click flag attempt.
func (c *cell) JawsClick(elem *jaws.Element, click jaws.Click) error {
	if click.Shift {
		elem.Request.Dirty(c.game.toggleFlag(c)...)
	} else {
		elem.Request.Dirty(c.game.clickCell(c)...)
	}
	return nil
}

// JawsContextMenu handles a flag attempt.
func (c *cell) JawsContextMenu(elem *jaws.Element, _ jaws.Click) error {
	elem.Request.Dirty(c.game.toggleFlag(c)...)
	return nil
}

// Board returns the fixed grid of cells for template iteration.
//
// Each Button reads its cell directly under the game lock when it renders.
func (g *game) Board() [][]*cell {
	return g.cells
}

// Status returns dynamic status text with precise dependencies.
func (g *game) Status() bind.Getter[string] {
	return bind.StringGetterFunc(func(*jaws.Element) string {
		return g.statusText()
	}, &g.started, &g.gameOver, &g.won)
}

// Stats returns dynamic game statistics with precise dependencies.
func (g *game) Stats() bind.Getter[string] {
	return bind.StringGetterFunc(func(*jaws.Element) string {
		return g.statsText()
	}, &g.revealed, &g.flags)
}

// NewGameAction returns the semantic action for the New game Button.
func (g *game) NewGameAction() ui.Object {
	return ui.New("New game").Clicked(func(_ ui.Object, elem *jaws.Element, _ jaws.Click) error {
		elem.Request.Dirty(g.reset()...)
		return nil
	})
}

func (g *game) statusText() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch {
	case g.gameOver && g.won:
		return "The shared board is clear."
	case g.gameOver:
		return "A mine was revealed."
	case !g.started:
		return "Click reveals. Right-click or Shift-click toggles flags. The first reveal is safe."
	default:
		return "Click reveals. Right-click or Shift-click toggles flags."
	}
}

func (g *game) statsText() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	remaining := g.rows*g.cols - g.mines - g.revealed
	return fmt.Sprintf("Mines: %d | Flags: %d | Safe cells left: %d", g.mines, g.flags, remaining)
}
