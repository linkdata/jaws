package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Div renders an HTML div element with dynamic inner HTML.
type Div struct{ HTMLInner }

// NewDiv returns a div widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewDiv(innerHTML any) *Div { return &Div{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

// JawsRender renders ui as an HTML div element.
func (u *Div) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "div", "", params)
}

// Div renders an HTML div element.
//
// A plain string innerHTML is trusted HTML and is not escaped; see [NewDiv] and
// [bind.MakeHTMLGetter] for how to pass untrusted user input safely.
func (rw RequestWriter) Div(innerHTML any, params ...any) error {
	return rw.NewUI(NewDiv(innerHTML), params...)
}
