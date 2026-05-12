package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type A struct{ HTMLInner }

// NewA returns an anchor widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to bind.MakeHTMLGetter; plain strings are trusted HTML.
func NewA(innerHTML any) *A { return &A{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

func (ui *A) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "a", "", params)
}

func (rw RequestWriter) A(innerHTML any, params ...any) error {
	return rw.UI(NewA(innerHTML), params...)
}
