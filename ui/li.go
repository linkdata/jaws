package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Li struct{ HTMLInner }

func NewLi(innerHTML core.HTMLGetter) *Li { return &Li{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Li) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}
