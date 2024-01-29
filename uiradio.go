package jaws

import (
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "radio", params...)
}

func NewUiRadio(vp BoolSetter) *UiRadio {
	return &UiRadio{
		UiInputBool{
			BoolSetter: vp,
		},
	}
}

func (rq RequestWriter) Radio(value any, params ...any) error {
	return rq.UI(NewUiRadio(makeBoolSetter(value)), params...)
}
