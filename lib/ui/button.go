package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Button renders an HTML button element with dynamic inner HTML.
type Button struct{ HTMLInner }

// NewButton returns a button widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewButton(innerHTML any) *Button {
	return &Button{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

// JawsRender renders ui as an HTML button element.
func (u *Button) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "button", "button", params)
}

// Button renders an HTML button element.
func (rw RequestWriter) Button(innerHTML any, params ...any) error {
	return rw.UI(NewButton(innerHTML), params...)
}
