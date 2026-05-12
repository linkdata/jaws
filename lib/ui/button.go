package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Button struct{ HTMLInner }

// NewButton returns a button widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to bind.MakeHTMLGetter; plain strings are trusted HTML.
func NewButton(innerHTML any) *Button {
	return &Button{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

func (ui *Button) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "button", "button", params)
}

func (rw RequestWriter) Button(innerHTML any, params ...any) error {
	return rw.UI(NewButton(innerHTML), params...)
}
