package jaws

import (
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderBoolInput(e, w, "radio", params...)
}

func NewUiRadio(vp BoolSetter) *UiRadio {
	return &UiRadio{
		UiInputBool{
			BoolSetter: vp,
		},
	}
}

func (rq *Request) Radio(value interface{}, params ...interface{}) error {
	return rq.UI(NewUiRadio(makeBoolSetter(value)), params...)
}
