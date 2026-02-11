package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Span struct{ HTMLInner }

func NewSpan(innerHTML core.HTMLGetter) *Span { return &Span{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Span) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}
