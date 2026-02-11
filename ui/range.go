package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Range struct{ InputFloat }

func NewRange(g core.Setter[float64]) *Range { return &Range{InputFloat{Setter: g}} }
func (ui *Range) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "range", params...)
}
