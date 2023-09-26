package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderBoolInput(e, w, "checkbox", params...)
}

func NewUiCheckbox(g BoolGetter) *UiCheckbox {
	return &UiCheckbox{
		UiInputBool{
			BoolGetter: g,
		},
	}
}

func (rq *Request) Checkbox(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiCheckbox(makeBoolGetter(value)), params...)
}
