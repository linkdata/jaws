package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Li renders an HTML list item with dynamic inner HTML.
type Li struct{ HTMLInner }

// NewLi returns a list item widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewLi(innerHTML any) *Li { return &Li{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }

// JawsRender renders ui as an HTML list item.
func (u *Li) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "li", "", params)
}

// Li renders an HTML list item.
func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	return rw.UI(NewLi(innerHTML), params...)
}
