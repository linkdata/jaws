package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Div struct{ HTMLInner }

func NewDiv(innerHTML any) *Div { return &Div{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }
func (rw RequestWriter) Div(innerHTML any, params ...any) error {
	return rw.UI(NewDiv(innerHTML), params...)
}

func (ui *Div) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}
