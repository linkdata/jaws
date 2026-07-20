package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Button renders an HTML button element with dynamic inner HTML.
//
// One Button value may back multiple live [jaws.Element] values. Its HTML getter
// is shared by those Elements and must be safe for their render, update and event
// calls.
type Button struct{ HTMLInner }

// NewButton returns a button widget whose inner HTML is rendered from innerHTML.
//
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewButton(innerHTML any) *Button {
	return &Button{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

// JawsRender renders ui as an HTML button element.
func (u *Button) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "button", "button", params)
}

// Button renders an HTML button element. A plain string innerHTML is trusted HTML;
// see [NewButton] and [bind.MakeHTMLGetter] to pass untrusted input safely.
func (rw RequestWriter) Button(innerHTML any, params ...any) error {
	return rw.NewUI(NewButton(innerHTML), params...)
}
