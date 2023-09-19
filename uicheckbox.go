package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	ui.UiInputBool.WriteHtmlInput(e, w, "checkbox", params...)
}

func NewUiCheckbox(vp ValueProxy) (ui *UiCheckbox) {
	ui = &UiCheckbox{
		UiInputBool: UiInputBool{
			UiInput: NewUiInput(vp),
		},
	}
	return
}

func (rq *Request) Checkbox(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiCheckbox(MakeValueProxy(value)), params...)
}
