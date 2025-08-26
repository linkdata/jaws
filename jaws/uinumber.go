package jaws

import (
	"io"
)

type UiNumber struct {
	UiInputFloat
}

func (ui *UiNumber) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "number", params...)
}

func NewUiNumber(g Setter[float64]) *UiNumber {
	return &UiNumber{
		UiInputFloat{
			Setter: g,
		},
	}
}

func (rq RequestWriter) Number(value any, params ...any) error {
	return rq.UI(NewUiNumber(makeSetterFloat64(value)), params...)
}
