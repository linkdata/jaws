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

func (rq *Request) Text(tagstring, val string, attrs ...interface{}) template.HTML {
	ui := &UiText{
		UiInputText: UiInputText{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: StringTags(tagstring)},
			},
			Value: val,
		},
	}
	return rq.UI(ui, attrs...)
}
