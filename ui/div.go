package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Div struct{ HTMLInner }

func NewDiv(innerHTML core.HTMLGetter) *Div { return &Div{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Div) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}
