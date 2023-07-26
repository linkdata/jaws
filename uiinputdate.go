package jaws

import (
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiBase
	Value       time.Time
	InputDateFn InputDateFn
}

func (ui *UiInputDate) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return WriteHtmlInput(w, jid, "date", ui.Value.Format(ISO8601), attrs...)
}

func (ui *UiInputDate) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input && ui.InputDateFn != nil {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(ISO8601, val); err != nil {
				return
			}
		}
		err = ui.InputDateFn(rq, jid, v)
	}
	return
}
