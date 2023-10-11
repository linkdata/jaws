package jaws

import (
	"io"
	"time"

	"github.com/linkdata/jaws/what"
)

type UiInputDate struct {
	UiInput
	TimeGetter
}

func (ui *UiInputDate) str() string {
	return ui.Last.Load().(time.Time).Format(ISO8601)
}

func (ui *UiInputDate) renderDateInput(e *Element, w io.Writer, jid Jid, htmltype string, params ...interface{}) {
	ui.parseGetter(e, ui.TimeGetter)
	attrs := ui.parseParams(e, params)
	ui.Last.Store(ui.JawsGetTime(e))
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, ui.str(), attrs...))
}

func (ui *UiInputDate) JawsUpdate(e *Element) {
	if t := ui.JawsGetTime(e); ui.Last.Swap(t) != t {
		e.SetValue(ui.str())
	}
}

func (ui *UiInputDate) JawsEvent(e *Element, wht what.What, val string) (stop bool, err error) {
	if wht == what.Input {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(ISO8601, val); err != nil {
				return
			}
		}
		ui.Last.Store(v)
		err = ui.TimeGetter.(TimeSetter).JawsSetTime(e, v)
		e.Dirty(ui.Tag)
		if err != nil {
			return
		}
	}
	return ui.UiHtml.JawsEvent(e, wht, val)
}
