package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Checkbox struct{ InputBool }

func NewCheckbox(g bind.Setter[bool]) *Checkbox { return &Checkbox{InputBool{Setter: g}} }
func (rw RequestWriter) Checkbox(value any, params ...any) error {
	return rw.UI(NewCheckbox(bind.MakeSetter[bool](value)), params...)
}

func (ui *Checkbox) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "checkbox", params...)
}
