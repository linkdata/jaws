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

func (rq *Request) Range(tagstring string, val interface{}, attrs ...interface{}) template.HTML {
	ui := &UiRange{
		UiInputFloat: UiInputFloat{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: StringTags(tagstring)},
			},
		},
	}
	ui.ProcessValue(val)
	return rq.UI(ui, attrs...)
}