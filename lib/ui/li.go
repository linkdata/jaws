package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Li struct{ HTMLInner }

// NewLi returns a list item widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to bind.MakeHTMLGetter; plain strings are trusted HTML.
func NewLi(innerHTML any) *Li { return &Li{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

func (ui *Li) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}

func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	return rw.UI(NewLi(innerHTML), params...)
}
