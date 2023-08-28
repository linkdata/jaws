package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputBool.WriteHtmlInput(e, w, "checkbox", e.Data)
}

func NewUiCheckbox(up Params) (ui *UiCheckbox) {
	ui = &UiCheckbox{
		UiInputBool: UiInputBool{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Checkbox(params ...interface{}) template.HTML {
	return rq.UI(NewUiCheckbox(NewParams(params)), params...)
}
