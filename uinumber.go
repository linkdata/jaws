package jaws

import (
	"html/template"
	"io"
)

type UiNumber struct {
	UiInputFloat
}

func (ui *UiNumber) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputFloat.WriteHtmlInput(rq, w, "number", jid, data...)
}

func NewUiNumber(tags []interface{}, val interface{}) (ui *UiNumber) {
	ui = &UiNumber{
		UiInputFloat: UiInputFloat{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: tags},
			},
		},
	}
	ui.ProcessValue(val)
	return
}

func (rq *Request) Number(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiNumber(ProcessTags(tagitem), val), attrs...)
}
