package jaws

import (
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiInput
	TimeSetter
}

func (ui *UiInputDate) str() string {
	return ui.Last.Load().(time.Time).Format(ISO8601)
}

func (ui *UiInputDate) renderDateInput(e *Element, w io.Writer, htmltype string, params ...any) error {
	ui.applyGetter(e, ui.TimeSetter)
	attrs := e.ApplyParams(params)
	ui.Last.Store(ui.JawsGetTime(e))
	return WriteHtmlInput(w, e.Jid(), htmltype, ui.str(), attrs)
}

func (ui *UiInputDate) JawsUpdate(e *Element) {
	if t := ui.JawsGetTime(e); ui.Last.Swap(t) != t {
		e.SetValue(ui.str())
	}
}

func (ui *UiInputDate) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(ISO8601, val); err != nil {
				return
			}
		}
		err = ui.maybeDirty(v, e, ui.TimeSetter.JawsSetTime(e, v))
	}
	return
}
