package jaws

import (
	"html/template"
	"io"
)

type UiPassword struct {
	UiInputText
}

func (ui *UiPassword) JawsRender(e *Element, w io.Writer) {
	ui.UiInputText.WriteHtmlInput(e, w, "password")
}

func NewUiPassword(up Params) (ui *UiPassword) {
	ui = &UiPassword{
		UiInputText: UiInputText{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Password(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiPassword(NewParams(value, params)), params...)
}
