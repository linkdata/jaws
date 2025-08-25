package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiInput
	Setter[float64]
}

func (ui *UiInputFloat) str() string {
	return strconv.FormatFloat(ui.Last.Load().(float64), 'f', -1, 64)
}

func (ui *UiInputFloat) renderFloatInput(e ElementIf, w io.Writer, htmltype string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		ui.Last.Store(ui.JawsGet(e))
		err = WriteHTMLInput(w, e.Jid(), htmltype, ui.str(), attrs)
	}
	return
}

func (ui *UiInputFloat) JawsUpdate(e ElementIf) {
	if f := ui.JawsGet(e); ui.Last.Swap(f) != f {
		e.SetValue(ui.str())
	}
}

func (ui *UiInputFloat) JawsEvent(e ElementIf, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		err = ui.MaybeDirty(v, e, ui.Setter.JawsSet(e, v))
	}
	return
}
