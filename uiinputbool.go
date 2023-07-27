package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiHtml
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
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		old := ui.Value
		ui.Value = v
		if ui.InputBoolFn != nil {
			if err = ui.InputBoolFn(rq, jid, ui.Value); err != nil {
				ui.Value = old
			}
		}
	}
	return
}
