package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Range struct{ InputFloat }

func NewRange(g pkg.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }
func (ui *Range) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "range", params...)
}
