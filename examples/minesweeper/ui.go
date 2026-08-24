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

func (c *cell) stateLocked() string {
	switch {
	case c.revealed && c.mine:
		return "mine"
	case c.revealed:
		return "revealed"
	case c.flagged:
		return "flagged"
	default:
		return "hidden"
	}
}

func (c *cell) attributesLocked() (state, label string, disabled bool) {
	state = c.stateLocked()
	label = c.labelLocked()
	disabled = c.revealed || c.game.gameOver
	return
}

func (c *cell) initialAttrsLocked() (attrs template.HTMLAttr) {
	state, label, disabled := c.attributesLocked()
	attrs = htmlio.Attr("data-state", state)
	attrs += " " + htmlio.Attr("aria-label", label)
	if disabled {
		attrs += " " + htmlio.Attr("disabled", "disabled")
	}
	return
}

func setCellAttributes(elem *jaws.Element, state, label string, disabled bool) {
	elem.SetAttr("data-state", state)
	elem.SetAttr("aria-label", label)
	if disabled {
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
	inner := c.htmlLocked()
	state, label, disabled := c.attributesLocked()
	c.game.mu.Unlock()

	// Initial attributes and content are separate synchronized reads. Queueing
	// the current attributes here lets TailHTML reconcile the initial Button;
	// dirty updates use this same direct read of the authoritative cell.
	setCellAttributes(elem, state, label, disabled)
	return inner
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
