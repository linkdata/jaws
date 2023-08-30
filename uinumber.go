package jaws

import (
	"html/template"
	"io"
)

type UiNumber struct {
	UiInputFloat
}

func (ui *UiNumber) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputFloat.WriteHtmlInput(e, w, "number")
}

func NewUiNumber(up Params) (ui *UiNumber) {
	ui = &UiNumber{
		UiInputFloat: UiInputFloat{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Number(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiNumber(NewParams(value, params)), params...)
}
