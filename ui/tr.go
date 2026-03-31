package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type Tr struct{ HTMLInner }

func NewTr(innerHTML bind.HTMLGetter) *Tr { return &Tr{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Tr(innerHTML any, params ...any) error {
	return rw.UI(NewTr(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Tr) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "tr", "", params)
}
