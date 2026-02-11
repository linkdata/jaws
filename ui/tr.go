package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Tr struct{ HTMLInner }

func NewTr(innerHTML pkg.HTMLGetter) *Tr { return &Tr{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Tr) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "tr", "", params)
}
