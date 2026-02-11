package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Checkbox struct{ InputBool }

func NewCheckbox(g pkg.Setter[bool]) *Checkbox { return &Checkbox{InputBool{Setter: g}} }
func (ui *Checkbox) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "checkbox", params...)
}
