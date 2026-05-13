package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// A renders an HTML anchor element with dynamic inner HTML.
type A struct{ HTMLInner }

// NewA returns an anchor widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewA(innerHTML any) *A { return &A{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

// JawsRender renders ui as an HTML anchor element.
func (u *A) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "a", "", params)
}

// A renders an HTML anchor element.
func (rw RequestWriter) A(innerHTML any, params ...any) error {
	return rw.UI(NewA(innerHTML), params...)
}
