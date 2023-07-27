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

func (rq *Request) Radio(tagstring string, val bool, fn InputBoolFn, attrs ...interface{}) template.HTML {
	ui := &UiRadio{
		UiInputBool: UiInputBool{
			UiHtml:      UiHtml{Tags: StringTags(tagstring)},
			Value:       val,
			InputBoolFn: fn,
		},
	}
	return rq.UI(ui, attrs...)
}
