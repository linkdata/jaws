package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiInput
}

func (ui *UiInputFloat) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	if val, ok := ui.Get().(float64); ok {
		return ui.UiInput.WriteHtmlInput(rq, w, htmltype, strconv.FormatFloat(val, 'f', -1, 64), jid, data...)
	}
	panic("jaws: UiInputFloat: not float64")
}

func (ui *UiInputFloat) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(rq, wht, jid, val)
	}
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		err = ui.Set(v)
	}
	return
}
