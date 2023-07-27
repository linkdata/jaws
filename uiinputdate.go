package jaws

import (
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiHtml
	Value       time.Time
	InputDateFn InputDateFn
}

func (ui *UiInputDate) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	return ui.UiHtml.WriteHtmlInput(rq, w, htmltype, ui.Value.Format(ISO8601), jid, data...)
}

func (ui *UiInputDate) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(ISO8601, val); err != nil {
				return
			}
		}
		old := ui.Value
		ui.Value = v
		if ui.InputDateFn != nil {
			if err = ui.InputDateFn(rq, jid, ui.Value); err != nil {
				ui.Value = old
			}
		}
	}
	return
}
