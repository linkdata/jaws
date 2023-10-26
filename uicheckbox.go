package jaws

import (
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderBoolInput(e, w, "checkbox", params...)
}

func NewUiCheckbox(g BoolSetter) *UiCheckbox {
	return &UiCheckbox{
		UiInputBool{
			BoolSetter: g,
		},
	}
}

func (rq *Request) Checkbox(value interface{}, params ...interface{}) error {
	return rq.UI(NewUiCheckbox(makeBoolSetter(value)), params...)
}
