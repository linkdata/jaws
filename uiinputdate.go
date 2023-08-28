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

func (ui *UiInputDate) WriteHtmlInput(e *Element, w io.Writer, jid Jid, htmltype string, data ...interface{}) error {
	val := ui.Get(e)
	if t, ok := val.(time.Time); ok {
		if t.IsZero() {
			t = time.Now()
			ui.Set(e, t)
		}
		writeUiDebug(e, w)
		return ui.UiInput.WriteHtmlInput(w, jid, htmltype, t.Format(ISO8601), data...)
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
		ui.Set(e, v)
	}
	return
}
