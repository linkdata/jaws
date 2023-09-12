package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer) {
	ui.UiInputBool.WriteHtmlInput(e, w, "checkbox", e.Attrs())
}

func NewUiCheckbox(up Params) (ui *UiCheckbox) {
	ui = &UiCheckbox{
		UiInputBool: UiInputBool{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Checkbox(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiCheckbox(NewParams(value, params)), params...)
}
