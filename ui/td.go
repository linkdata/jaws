package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Td struct{ HTMLInner }

func NewTd(innerHTML core.HTMLGetter) *Td { return &Td{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Td) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}
