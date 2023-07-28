package jaws

import (
	"html/template"
	"io"
)

type UiRange struct {
	UiInputFloat
}

func (ui *UiRange) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputFloat.WriteHtmlInput(rq, w, "range", jid, data...)
}

func NewUiRange(tags []interface{}, val interface{}) (ui *UiRange) {
	ui = &UiRange{
		UiInputFloat: UiInputFloat{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: tags},
			},
		},
	}
	ui.ProcessValue(val)
	return
}

func (rq *Request) Range(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiRange(ProcessTags(tagitem), val), attrs...)
}
