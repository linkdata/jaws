package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Span struct{ HTMLInner }

func NewSpan(innerHTML pkg.HTMLGetter) *Span { return &Span{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Span) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}
