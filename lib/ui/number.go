package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Number renders an HTML number input bound to a float64 setter.
type Number struct{ InputFloat }

// NewNumber returns a number input widget bound to g.
func NewNumber(g bind.Setter[float64]) *Number { return &Number{InputFloat{Setter: g}} }

// JawsRender renders ui as an HTML number input.
func (u *Number) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return u.renderFloatInput(e, w, "number", params...)
}

// Number renders an HTML number input.
func (rw RequestWriter) Number(value any, params ...any) error {
	return rw.UI(NewNumber(bind.MakeSetterFloat64(value)), params...)
}
