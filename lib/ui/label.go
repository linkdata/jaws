package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Label renders an HTML label element with dynamic inner HTML.
type Label struct{ HTMLInner }

// NewLabel returns a label widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewLabel(innerHTML any) *Label {
	return &Label{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

// JawsRender renders ui as an HTML label element.
func (u *Label) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(e, w, "label", "", params)
}

// Label renders an HTML label element.
func (rw RequestWriter) Label(innerHTML any, params ...any) error {
	return rw.UI(NewLabel(innerHTML), params...)
}
