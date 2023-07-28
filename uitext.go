package jaws

import (
	"html/template"
	"io"
)

type UiText struct {
	UiInputText
}

func (ui *UiText) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputText.WriteHtmlInput(rq, w, "text", jid, data...)
}

func NewUiText(tags []interface{}, val interface{}) (ui *UiText) {
	ui = &UiText{
		UiInputText: UiInputText{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: tags},
			},
		},
	}
	ui.ProcessValue(val)
	return
}

func (rq *Request) Text(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiText(ProcessTags(tagitem), val), attrs...)
}
