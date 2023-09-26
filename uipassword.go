package jaws

import (
	"html/template"
	"io"
)

type UiPassword struct {
	UiInputText
}

func (ui *UiPassword) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderStringInput(e, w, "password", params...)
}

func NewUiPassword(g StringGetter) *UiPassword {
	return &UiPassword{
		UiInputText{
			StringGetter: g,
		},
	}
}

func (rq *Request) Password(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiPassword(makeStringGetter(value)), params...)
}
