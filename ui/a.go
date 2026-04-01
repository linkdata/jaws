package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/bind"
)

type A struct{ HTMLInner }

func NewA(innerHTML bind.HTMLGetter) *A { return &A{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) A(innerHTML any, params ...any) error {
	return rw.UI(NewA(bind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *A) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "a", "", params)
}
