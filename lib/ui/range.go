package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Range renders an HTML range input bound to a float64 setter.
//
// A Range value must back at most one live [jaws.Element]. Construct distinct
// Range values over the same setter to render one bound value more than once.
type Range struct{ InputFloat }

// NewRange returns a range input widget bound to g.
//
// The bound value must be finite. A non-finite value (NaN or ±Inf) has no valid
// rendering or wire representation, so rendering, updating, or receiving one from
// the browser cancels the [jaws.Request] with a cause wrapping
// [jaws.ErrValueNotFinite].
//
// The widget emits no min, max, or step attribute of its own, so the browser
// applies the HTML range defaults min="0", max="100", step="1". A bound value
// outside that domain, or off the step grid, is silently clamped and rounded by
// the browser, and the adjusted value is echoed back through the setter on the
// next input event. Supply explicit min, max, and step attributes as render
// params when the defaults do not cover the bound value's domain.
func NewRange(g bind.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }

// JawsRender renders ui as an HTML range input.
func (u *Range) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderFloatInput(elem, w, "range", params...)
}

// Range renders an HTML range input.
//
// Supply min, max, and step attributes in params when the browser defaults
// (min="0", max="100", step="1") do not cover the bound value's domain; a value
// outside it is silently clamped and rounded by the browser. For example:
//
//	rw.Range(&v, `min="0"`, `max="200"`, `step="0.5"`)
//
// See [NewRange] for details and for how a non-finite bound value is handled.
func (rw RequestWriter) Range(value any, params ...any) error {
	return rw.NewUI(NewRange(bind.MakeSetterFloat64(value)), params...)
}
