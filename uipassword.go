package jaws

import (
	"html/template"
	"io"
)

type UiPassword struct {
	UiInputText
}

func (ui *UiPassword) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputText.WriteHtmlInput(e, w, "password")
}

func NewUiPassword(up Params) (ui *UiPassword) {
	ui = &UiPassword{
		UiInputText: UiInputText{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Password(params ...interface{}) template.HTML {
	return rq.UI(NewUiPassword(NewParams(params)), params...)
}
