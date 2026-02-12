package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Td struct{ HTMLInner }

func NewTd(innerHTML core.HTMLGetter) *Td { return &Td{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Td(innerHTML any, params ...any) error {
	return rw.UI(NewTd(core.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Td) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}
