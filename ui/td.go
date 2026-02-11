package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Td struct{ HTMLInner }

func NewTd(innerHTML pkg.HTMLGetter) *Td { return &Td{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Td) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}
