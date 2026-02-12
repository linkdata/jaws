package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Li struct{ HTMLInner }

func NewLi(innerHTML core.HTMLGetter) *Li { return &Li{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	return rw.UI(NewLi(core.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Li) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}
