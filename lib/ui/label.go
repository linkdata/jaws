package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Label struct{ HTMLInner }

// NewLabel returns a label widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to bind.MakeHTMLGetter; plain strings are trusted HTML.
func NewLabel(innerHTML any) *Label {
	return &Label{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

func (ui *Label) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "label", "", params)
}

func (rw RequestWriter) Label(innerHTML any, params ...any) error {
	return rw.UI(NewLabel(innerHTML), params...)
}
