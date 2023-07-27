package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiHtml
	HtmlType    string
	Value       bool
	InputBoolFn InputBoolFn
}

func (ui *UiInputBool) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	if ui.Value {
		data = append(data, "checked")
	}
	return ui.UiHtml.WriteHtmlInput(rq, w, htmltype, "", jid, data...)
}

func (ui *UiInputBool) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input && ui.InputBoolFn != nil {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		err = ui.InputBoolFn(rq, jid, v)
	}
	return
}
