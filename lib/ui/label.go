package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Label renders an HTML label element with dynamic inner HTML.
//
// One Label value may back multiple live [jaws.Element] values. Its HTML getter
// is shared by those Elements and must be safe for their render, update and event
// calls.
type Label struct{ HTMLInner }

// NewLabel returns a label widget whose inner HTML is rendered from innerHTML.
//
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewLabel(innerHTML any) *Label {
	return &Label{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

// JawsRender renders ui as an HTML label element.
func (u *Label) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "label", "", params)
}

// Label renders an HTML label element. A plain string innerHTML is trusted HTML;
// see [NewLabel] and [bind.MakeHTMLGetter] to pass untrusted input safely.
func (rw RequestWriter) Label(innerHTML any, params ...any) error {
	return rw.NewUI(NewLabel(innerHTML), params...)
}
