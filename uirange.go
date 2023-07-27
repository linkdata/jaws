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

func (rq *Request) Range(tagstring string, val float64, fn InputFloatFn, attrs ...interface{}) template.HTML {
	ui := &UiRange{
		UiInputFloat: UiInputFloat{
			UiHtml:       UiHtml{Tags: StringTags(tagstring)},
			Value:        val,
			InputFloatFn: fn,
		},
	}
	return rq.UI(ui, attrs...)
}
