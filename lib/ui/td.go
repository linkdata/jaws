package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Td struct{ HTMLInner }

// NewTd returns a table cell widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to bind.MakeHTMLGetter; plain strings are trusted HTML.
func NewTd(innerHTML any) *Td { return &Td{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

func (ui *Td) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}

func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	return rw.UI(NewTd(innerHTML), params...)
}
