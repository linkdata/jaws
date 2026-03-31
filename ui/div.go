package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type Div struct{ HTMLInner }

func NewDiv(innerHTML bind.HTMLGetter) *Div { return &Div{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Div(innerHTML any, params ...any) error {
	return rw.UI(NewDiv(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Div) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}
