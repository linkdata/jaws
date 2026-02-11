package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Button struct{ HTMLInner }

func NewButton(innerHTML core.HTMLGetter) *Button { return &Button{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Button) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "button", "button", params)
}
