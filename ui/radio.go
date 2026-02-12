package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Radio struct{ InputBool }

func NewRadio(vp core.Setter[bool]) *Radio { return &Radio{InputBool{Setter: vp}} }
func (rw RequestWriter) Radio(value any, params ...any) error {
	return rw.UI(NewRadio(core.MakeSetter[bool](value)), params...)
}

func (ui *Radio) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "radio", params...)
}
