package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Div struct{ HTMLInner }

func NewDiv(innerHTML bind.HTMLGetter) *Div { return &Div{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Div(innerHTML any, params ...any) error {
	return rw.UI(NewDiv(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Div) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}
