package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Checkbox struct{ InputBool }

func NewCheckbox(g core.Setter[bool]) *Checkbox { return &Checkbox{InputBool{Setter: g}} }
func (rw RequestWriter) Checkbox(value any, params ...any) error {
	return rw.UI(NewCheckbox(core.MakeSetter[bool](value)), params...)
}

func (ui *Checkbox) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "checkbox", params...)
}
