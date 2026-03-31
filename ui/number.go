package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
)

type Number struct{ InputFloat }

func NewNumber(g jawsbind.Setter[float64]) *Number { return &Number{InputFloat{Setter: g}} }
func (rw RequestWriter) Number(value any, params ...any) error {
	return rw.UI(NewNumber(jawsbind.MakeSetterFloat64(value)), params...)
}

func (ui *Number) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "number", params...)
}
