package jaws

import (
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderBoolInput(e, w, "checkbox", params...)
}

func NewUiCheckbox(g BoolSetter) *UiCheckbox {
	return &UiCheckbox{
		UiInputBool{
			BoolSetter: g,
		},
	}
}

func (rq RequestWriter) Checkbox(value any, params ...any) error {
	return rq.UI(NewUiCheckbox(makeBoolSetter(value)), params...)
}
