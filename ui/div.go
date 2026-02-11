package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Div struct{ HTMLInner }

func NewDiv(innerHTML pkg.HTMLGetter) *Div { return &Div{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Div) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}
