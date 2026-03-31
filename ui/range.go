package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
)

type Range struct{ InputFloat }

func NewRange(g jawsbind.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }
func (rw RequestWriter) Range(value any, params ...any) error {
	return rw.UI(NewRange(jawsbind.MakeSetterFloat64(value)), params...)
}

func (ui *Range) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "range", params...)
}
