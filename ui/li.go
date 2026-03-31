package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type Li struct{ HTMLInner }

func NewLi(innerHTML bind.HTMLGetter) *Li { return &Li{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	return rw.UI(NewLi(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Li) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}
