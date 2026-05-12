package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Checkbox struct{ InputBool }

// NewCheckbox returns a checkbox input widget bound to g.
func NewCheckbox(g bind.Setter[bool]) *Checkbox { return &Checkbox{InputBool{Setter: g}} }

func (ui *Checkbox) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "checkbox", params...)
}

func (rw RequestWriter) Checkbox(value any, params ...any) error {
	return rw.UI(NewCheckbox(bind.MakeSetter[bool](value)), params...)
}
