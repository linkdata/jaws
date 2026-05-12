package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Span struct{ HTMLInner }

// NewSpan returns a span widget whose inner HTML is rendered from innerHTML.
// innerHTML is passed to bind.MakeHTMLGetter; plain strings are trusted HTML.
func NewSpan(innerHTML any) *Span {
	return &Span{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}

func (ui *Span) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}

func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	return rw.UI(NewSpan(innerHTML), params...)
}
