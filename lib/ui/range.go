package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Range struct{ InputFloat }

func NewRange(g bind.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }
func (rw RequestWriter) Range(value any, params ...any) error {
	return rw.UI(NewRange(bind.MakeSetterFloat64(value)), params...)
}

func (ui *Range) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "range", params...)
}
