package jaws

import (
	"io"
)

type UiRange struct {
	UiInputFloat
}

func (ui *UiRange) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "range", params...)
}

func NewUiRange(g Setter[float64]) *UiRange {
	return &UiRange{
		UiInputFloat{
			Setter: g,
		},
	}
}

func (rq RequestWriter) Range(value any, params ...any) error {
	return rq.UI(NewUiRange(makeSetterFloat64(value)), params...)
}
