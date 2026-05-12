package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type A struct{ HTMLInner }

func NewA(innerHTML any) *A { return &A{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}} }
func (rw RequestWriter) A(innerHTML any, params ...any) error {
	return rw.UI(NewA(innerHTML), params...)
}

func (ui *A) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "a", "", params)
}
