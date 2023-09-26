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

func MakeUiPassword(g StringGetter) UiPassword {
	return UiPassword{
		UiInputText{
			StringGetter: g,
		},
	}
}

func (rq *Request) Password(value interface{}, params ...interface{}) template.HTML {
	ui := MakeUiPassword(makeStringGetter(value))
	return rq.UI(&ui, params...)
}
