package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
)

type Range struct{ InputFloat }

func NewRange(g jawsbind.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }
func (rw RequestWriter) Range(value any, params ...any) error {
	return rw.UI(NewRange(jawsbind.MakeSetterFloat64(value)), params...)
}

func (ui *Range) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "range", params...)
}
