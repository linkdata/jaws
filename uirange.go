package jaws

import (
	"html/template"
	"io"
)

type UiRange struct {
	UiInputFloat
}

func (ui *UiRange) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputFloat.WriteHtmlInput(e, w, "range")
}

func NewUiRange(up Params) (ui *UiRange) {
	ui = &UiRange{
		UiInputFloat: UiInputFloat{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Range(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiRange(NewParams(value, params)), params...)
}
