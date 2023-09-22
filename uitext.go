package jaws

import (
	"html/template"
	"io"
)

type UiText struct {
	UiInputText
}

func (ui *UiText) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiInputText.WriteHtmlInput(e, w, "text", params...)
}

func NewUiText(vp ValueProxy) (ui *UiText) {
	ui = &UiText{
		UiInputText{
			UiInput{
				ValueProxy: vp,
			},
		},
	}
	return
}

func (rq *Request) Text(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiText(MakeValueProxy(value)), params...)
}
