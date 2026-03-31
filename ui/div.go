package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
)

type Div struct{ HTMLInner }

func NewDiv(innerHTML jawsbind.HTMLGetter) *Div { return &Div{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Div(innerHTML any, params ...any) error {
	return rw.UI(NewDiv(jawsbind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Div) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}
