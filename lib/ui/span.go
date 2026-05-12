package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Span struct{ HTMLInner }

func NewSpan(innerHTML any) *Span {
	return &Span{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}
func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	return rw.UI(NewSpan(innerHTML), params...)
}

func (ui *Span) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}
