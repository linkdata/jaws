package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type Checkbox struct{ InputBool }

func NewCheckbox(g bind.Setter[bool]) *Checkbox { return &Checkbox{InputBool{Setter: g}} }
func (rw RequestWriter) Checkbox(value any, params ...any) error {
	return rw.UI(NewCheckbox(bind.MakeSetter[bool](value)), params...)
}

func (ui *Checkbox) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "checkbox", params...)
}
