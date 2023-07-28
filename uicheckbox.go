package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputBool.WriteHtmlInput(rq, w, "checkbox", jid, data...)
}

func NewUiCheckbox(tags []interface{}, val interface{}) (ui *UiCheckbox) {
	ui = &UiCheckbox{
		UiInputBool: UiInputBool{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: tags},
			},
		},
	}
	ui.ProcessValue(val)
	return
}

func (rq *Request) Checkbox(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiCheckbox(ProcessTags(tagitem), val), attrs...)
}
