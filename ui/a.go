package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type A struct{ HTMLInner }

func NewA(innerHTML pkg.HTMLGetter) *A { return &A{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *A) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "a", "", params)
}
