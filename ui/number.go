package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Number struct{ InputFloat }

func NewNumber(g core.Setter[float64]) *Number { return &Number{InputFloat{Setter: g}} }
func (rw RequestWriter) Number(value any, params ...any) error {
	return rw.UI(NewNumber(core.MakeSetterFloat64(value)), params...)
}

func (ui *Number) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "number", params...)
}
