package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Number renders an HTML number input bound to a float64 setter.
//
// A Number value must back at most one live [jaws.Element]. Construct distinct
// Number values over the same setter to render one bound value more than once.
type Number struct{ InputFloat }

// NewNumber returns a number input widget bound to g.
//
// The bound value must be finite. A non-finite value (NaN or ±Inf) has no valid
// rendering or wire representation, so rendering, updating, or receiving one from
// the browser cancels the [jaws.Request] with a cause wrapping
// [jaws.ErrValueNotFinite].
func NewNumber(g bind.Setter[float64]) *Number { return &Number{InputFloat{Setter: g}} }

// JawsRender renders ui as an HTML number input.
func (u *Number) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderFloatInput(elem, w, "number", params...)
}

// Number renders an HTML number input.
//
// See [NewNumber] for how a non-finite bound value is handled.
func (rw RequestWriter) Number(value any, params ...any) error {
	return rw.NewUI(NewNumber(bind.MakeSetterFloat64(value)), params...)
}
