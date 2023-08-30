package jaws

import (
	"html/template"
	"io"
)

type UiText struct {
	UiInputText
}

func (ui *UiText) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputText.WriteHtmlInput(e, w, "text")
}

func NewUiText(up Params) (ui *UiText) {
	ui = &UiText{
		UiInputText: UiInputText{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Text(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiText(NewParams(value, params)))
}
