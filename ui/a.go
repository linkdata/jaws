package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type A struct{ HTMLInner }

func NewA(innerHTML core.HTMLGetter) *A { return &A{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) A(innerHTML any, params ...any) error {
	return rw.UI(NewA(core.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *A) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "a", "", params)
}
