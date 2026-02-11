package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Radio struct{ InputBool }

func NewRadio(vp pkg.Setter[bool]) *Radio { return &Radio{InputBool{Setter: vp}} }
func (ui *Radio) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "radio", params...)
}
