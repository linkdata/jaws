package jaws

import (
	"fmt"
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiInput
}

func (ui *UiInputDate) WriteHtmlInput(e *Element, w io.Writer, htmltype, jid string, data ...interface{}) error {
	val := ui.Get(e)
	if t, ok := val.(time.Time); ok {
		return ui.UiInput.WriteHtmlInput(w, htmltype, t.Format(ISO8601), jid, data...)
	}
	return fmt.Errorf("jaws: UiInputDate: expected time.Time, got %T", val)
}

func (ui *UiInputDate) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request(), wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(ISO8601, val); err != nil {
				return
			}
		}
		err = ui.Set(e, v)
	}
	return
}
