package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/bind"
)

type Span struct{ HTMLInner }

func NewSpan(innerHTML bind.HTMLGetter) *Span { return &Span{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	return rw.UI(NewSpan(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Span) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}
