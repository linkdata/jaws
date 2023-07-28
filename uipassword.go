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

func NewUiPassword(tags []interface{}, val interface{}) (ui *UiPassword) {
	ui = &UiPassword{
		UiInputText: UiInputText{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: tags},
			},
		},
	}
	ui.ProcessValue(val)
	return
}

func (rq *Request) Password(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiPassword(ProcessTags(tagitem), val), attrs...)
}
