package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Span renders an HTML span element with dynamic inner HTML.
type Span struct{ HTMLInner }

// NewSpan returns a span widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to [bind.MakeHTMLGetter]; plain strings are trusted HTML.
func NewSpan(innerHTML any) *Span {
	return &Span{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

// JawsRender renders ui as an HTML span element.
func (u *Span) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderInner(elem, w, "span", "", params)
}

// Span renders an HTML span element.
//
// A plain string innerHTML is trusted HTML and is not escaped; see [NewSpan] and
// [bind.MakeHTMLGetter] for how to pass untrusted user input safely.
func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	return rw.NewUI(NewSpan(innerHTML), params...)
}
