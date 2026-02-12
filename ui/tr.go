package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Tr struct{ HTMLInner }

func NewTr(innerHTML core.HTMLGetter) *Tr { return &Tr{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Tr(innerHTML any, params ...any) error {
	return rw.UI(NewTr(core.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Tr) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "tr", "", params)
}
