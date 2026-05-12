package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Tr struct{ HTMLInner }

// NewTr returns a table row widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to bind.MakeHTMLGetter; plain strings are trusted HTML.
func NewTr(innerHTML any) *Tr { return &Tr{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

func (ui *Tr) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "tr", "", params)
}

func (rw RequestWriter) Tr(innerHTML any, params ...any) error {
	return rw.UI(NewTr(innerHTML), params...)
}
