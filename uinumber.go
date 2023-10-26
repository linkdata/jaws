package jaws

import (
	"io"
)

type UiNumber struct {
	UiInputFloat
}

func (ui *UiNumber) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderFloatInput(e, w, "number", params...)
}

func NewUiNumber(g FloatSetter) *UiNumber {
	return &UiNumber{
		UiInputFloat{
			FloatSetter: g,
		},
	}
}

func (rq RequestWriter) Number(value interface{}, params ...interface{}) error {
	return rq.UI(NewUiNumber(makeFloatSetter(value)), params...)
}
