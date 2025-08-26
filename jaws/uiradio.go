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

func NewUiRadio(vp Setter[bool]) *UiRadio {
	return &UiRadio{
		UiInputBool{
			Setter: vp,
		},
	}
}

func (rq RequestWriter) Radio(value any, params ...any) error {
	return rq.UI(NewUiRadio(makeSetter[bool](value)), params...)
}
