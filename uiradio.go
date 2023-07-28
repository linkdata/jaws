package jaws

import (
	"html/template"
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputBool.WriteHtmlInput(rq, w, "radio", jid, data...)
}

func NewUiRadio(tags []interface{}, val interface{}) (ui *UiRadio) {
	ui = &UiRadio{
		UiInputBool: UiInputBool{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: tags},
			},
		},
	}
	ui.ProcessValue(val)
	return
}

func (rq *Request) Radio(tagitem interface{}, val interface{}, data ...interface{}) template.HTML {
	return rq.UI(NewUiRadio(ProcessTags(tagitem), val), data...)
}
