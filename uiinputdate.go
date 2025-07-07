package jaws

import (
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiInput
	Setter[time.Time]
}

func (ui *UiInputDate) str() string {
	return ui.Last.Load().(time.Time).Format(ISO8601)
}

func (ui *UiInputDate) renderDateInput(e *Element, w io.Writer, htmltype string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		ui.Last.Store(ui.JawsGet(e))
		err = WriteHTMLInput(w, e.Jid(), htmltype, ui.str(), attrs)
	}
	return
}

func (ui *UiInputDate) JawsUpdate(e *Element) {
	if t := ui.JawsGet(e); ui.Last.Swap(t) != t {
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
		err = ui.maybeDirty(v, e, ui.Setter.JawsSet(e, v))
	}
	return
}
