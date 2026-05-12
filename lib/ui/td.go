package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Td renders an HTML table cell with dynamic inner HTML.
type Td struct{ HTMLInner }

// NewTd returns a table cell widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewTd(innerHTML any) *Td { return &Td{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

// JawsRender renders ui as an HTML table cell.
func (u *Td) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(e, w, "td", "", params)
}

// Td renders an HTML table cell.
func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	return rw.UI(NewTd(innerHTML), params...)
}
