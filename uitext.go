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

func NewUiText(vp StringGetter) (ui *UiText) {
	ui = &UiText{
		UiInputText{
			StringGetter: vp,
		},
	}
	return
}

func (rq *Request) Text(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiText(makeStringGetter(value)), params...)
}
