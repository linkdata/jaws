package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiBase
	HtmlType    string
	Value       bool
	InputBoolFn InputBoolFn
}

func (ui *UiInputBool) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	if ui.Value {
		attrs = append(attrs, "checked")
	}
	return WriteHtmlInput(w, jid, ui.HtmlType, "", attrs...)
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
