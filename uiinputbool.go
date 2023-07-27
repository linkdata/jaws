package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiInput
}

func (ui *UiInputBool) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	if val, ok := ui.Get().(bool); ok && val {
		data = append(data, "checked")
	}
	return ui.UiInput.WriteHtmlInput(rq, w, htmltype, "", jid, data...)
}

func (ui *UiInputBool) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(rq, wht, jid, val)
	}
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		err = ui.Set(v)
	}
	return
}
