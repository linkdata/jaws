package jaws

import (
	"html/template"
	"io"
)

type UiPassword struct {
	UiInputText
}

func (ui *UiPassword) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputText.WriteHtmlInput(rq, w, "password", jid, data...)
}

func (rq *Request) Password(tagstring string, val interface{}, attrs ...interface{}) template.HTML {
	ui := &UiPassword{
		UiInputText: UiInputText{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: StringTags(tagstring)},
			},
		},
	}
	ui.ProcessValue(val)
	return rq.UI(ui, attrs...)
}
