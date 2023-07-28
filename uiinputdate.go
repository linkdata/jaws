package jaws

import (
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiInput
}

func (ui *UiInputDate) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	if val, ok := ui.Get().(time.Time); ok {
		return ui.UiInput.WriteHtmlInput(rq, w, htmltype, val.Format(ISO8601), jid, data...)
	}
	panic("jaws: UiInputDate: not time.Time")
}

func (ui *UiInputDate) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(rq, wht, jid, val)
	}
	if wht == what.Input {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(ISO8601, val); err != nil {
				return
			}
		}
		err = ui.Set(v)
	}
	return
}
