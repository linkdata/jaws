package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Radio renders an HTML radio input bound to a bool setter.
type Radio struct{ InputBool }

// NewRadio returns a radio input widget bound to vp.
func NewRadio(vp bind.Setter[bool]) *Radio { return &Radio{InputBool{Setter: vp}} }

// JawsRender renders ui as an HTML radio input.
func (u *Radio) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderBoolInput(elem, w, "radio", params...)
}

// Radio renders an HTML radio input.
func (rw RequestWriter) Radio(value any, params ...any) error {
	return rw.UI(NewRadio(bind.MakeSetter[bool](value)), params...)
}
