package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Range renders an HTML range input bound to a float64 setter.
type Range struct{ InputFloat }

// NewRange returns a range input widget bound to g.
//
// A range control cannot represent a non-finite bound value. Per the WHATWG
// value-sanitization rules a missing, empty, or invalid range value is coerced
// to the control's default (the midpoint of its min and max), so a bound NaN or
// ±Inf displays as that finite position while the server value stays non-finite.
// The widget renders no unparseable literal for such values, but unlike
// [NewNumber] the display cannot be blank. Bind a finite value to a range, or use
// a [NewNumber] when non-finite values are possible.
func NewRange(g bind.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }

// JawsRender renders ui as an HTML range input.
func (u *Range) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderFloatInput(elem, w, "range", params...)
}

// Range renders an HTML range input.
func (rw RequestWriter) Range(value any, params ...any) error {
	return rw.NewUI(NewRange(bind.MakeSetterFloat64(value)), params...)
}
