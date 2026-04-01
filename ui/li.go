package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/bind"
)

type Li struct{ HTMLInner }

func NewLi(innerHTML bind.HTMLGetter) *Li { return &Li{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Li(innerHTML any, params ...any) error {
	return rw.UI(NewLi(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Li) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}
