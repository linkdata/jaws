package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiHtml
	HtmlType    string
	Value       string
	InputTextFn InputTextFn
}

func (ui *UiInputText) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	return ui.UiHtml.WriteHtmlInput(rq, w, htmltype, ui.Value, jid, data...)
}

func (ui *UiInputText) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input && ui.InputTextFn != nil {
		err = ui.InputTextFn(rq, jid, val)
	}
	return
}
