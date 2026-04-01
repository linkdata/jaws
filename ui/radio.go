package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/bind"
)

type Radio struct{ InputBool }

func NewRadio(vp bind.Setter[bool]) *Radio { return &Radio{InputBool{Setter: vp}} }
func (rw RequestWriter) Radio(value any, params ...any) error {
	return rw.UI(NewRadio(bind.MakeSetter[bool](value)), params...)
}

func (ui *Radio) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "radio", params...)
}
