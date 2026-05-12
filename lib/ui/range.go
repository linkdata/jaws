package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Range renders an HTML range input bound to a float64 setter.
type Range struct{ InputFloat }

// NewRange returns a range input widget bound to g.
func NewRange(g bind.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }

// JawsRender renders ui as an HTML range input.
func (u *Range) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return u.renderFloatInput(e, w, "range", params...)
}

// Range renders an HTML range input.
func (rw RequestWriter) Range(value any, params ...any) error {
	return rw.UI(NewRange(bind.MakeSetterFloat64(value)), params...)
}
