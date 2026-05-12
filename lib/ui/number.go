package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Number struct{ InputFloat }

// NewNumber returns a number input widget bound to g.
func NewNumber(g bind.Setter[float64]) *Number { return &Number{InputFloat{Setter: g}} }

func (ui *Number) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "number", params...)
}

func (rw RequestWriter) Number(value any, params ...any) error {
	return rw.UI(NewNumber(bind.MakeSetterFloat64(value)), params...)
}
