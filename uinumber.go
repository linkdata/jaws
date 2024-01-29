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

func NewUiNumber(g FloatSetter) *UiNumber {
	return &UiNumber{
		UiInputFloat{
			FloatSetter: g,
		},
	}
}

func (rq RequestWriter) Number(value any, params ...any) error {
	return rq.UI(NewUiNumber(makeFloatSetter(value)), params...)
}
