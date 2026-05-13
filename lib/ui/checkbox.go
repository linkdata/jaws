package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Checkbox renders an HTML checkbox input bound to a bool setter.
type Checkbox struct{ InputBool }

// NewCheckbox returns a checkbox input widget bound to g.
func NewCheckbox(g bind.Setter[bool]) *Checkbox { return &Checkbox{InputBool{Setter: g}} }

// JawsRender renders ui as an HTML checkbox input.
func (u *Checkbox) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderBoolInput(elem, w, "checkbox", params...)
}

// Checkbox renders an HTML checkbox input.
func (rw RequestWriter) Checkbox(value any, params ...any) error {
	return rw.UI(NewCheckbox(bind.MakeSetter[bool](value)), params...)
}
