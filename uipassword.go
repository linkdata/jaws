package jaws

import (
	"html/template"
	"io"
)

type UiPassword struct {
	UiInputText
}

func (ui *UiPassword) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiInputText.WriteHtmlInput(e, w, "password", params...)
}

func NewUiPassword(vp ValueProxy) (ui *UiPassword) {
	ui = &UiPassword{
		UiInputText{
			UiInput{
				ValueProxy: vp,
			},
		},
	}
	return
}

func (rq *Request) Password(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiPassword(MakeValueProxy(value)), params...)
}
