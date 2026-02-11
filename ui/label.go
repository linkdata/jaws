package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Label struct{ HTMLInner }

func NewLabel(innerHTML core.HTMLGetter) *Label { return &Label{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Label) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "label", "", params)
}
