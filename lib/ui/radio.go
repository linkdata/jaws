package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Radio struct{ InputBool }

// NewRadio returns a radio input widget bound to vp.
func NewRadio(vp bind.Setter[bool]) *Radio { return &Radio{InputBool{Setter: vp}} }

func (ui *Radio) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "radio", params...)
}

func (rw RequestWriter) Radio(value any, params ...any) error {
	return rw.UI(NewRadio(bind.MakeSetter[bool](value)), params...)
}
