package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
)

type Span struct{ HTMLInner }

func NewSpan(innerHTML jawsbind.HTMLGetter) *Span { return &Span{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Span(innerHTML any, params ...any) error {
	return rw.UI(NewSpan(jawsbind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Span) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}
