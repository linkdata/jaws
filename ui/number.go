package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Number struct{ InputFloat }

func NewNumber(g pkg.Setter[float64]) *Number { return &Number{InputFloat{Setter: g}} }
func (ui *Number) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "number", params...)
}
