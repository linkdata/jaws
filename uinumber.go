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

func (rq *Request) Number(tagstring string, val interface{}, attrs ...interface{}) template.HTML {
	ui := &UiNumber{
		UiInputFloat: UiInputFloat{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: StringTags(tagstring)},
			},
		},
	}
	ui.ProcessValue(val)
	return rq.UI(ui, attrs...)
}
