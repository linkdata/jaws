package jaws

import (
	"html/template"
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputBool.WriteHtmlInput(e, w, "radio")
}

func NewUiRadio(up Params) (ui *UiRadio) {
	ui = &UiRadio{
		UiInputBool: UiInputBool{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Radio(params ...interface{}) template.HTML {
	return rq.UI(NewUiRadio(NewParams(params)), params...)
}
