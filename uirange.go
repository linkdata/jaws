package jaws

import (
	"html/template"
	"io"
)

type UiRange struct {
	UiInputFloat
}

func (ui *UiRange) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	ui.UiInputFloat.WriteHtmlInput(e, w, "range", params...)
}

func NewUiRange(vp ValueProxy) (ui *UiRange) {
	ui = &UiRange{
		UiInputFloat: UiInputFloat{
			UiInput: NewUiInput(vp),
		},
	}
	return
}

func (rq *Request) Range(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiRange(MakeValueProxy(value)), params...)
}
