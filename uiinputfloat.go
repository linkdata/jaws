package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiInput
	FloatSetter
}

func (ui *UiInputFloat) str() string {
	return strconv.FormatFloat(ui.Last.Load().(float64), 'f', -1, 64)
}

func (ui *UiInputFloat) renderFloatInput(e *Element, w io.Writer, htmltype string, params ...any) error {
	ui.applyGetter(e, ui.FloatSetter)
	attrs := e.ApplyParams(params)
	ui.Last.Store(ui.JawsGetFloat(e))
	return WriteHtmlInput(w, e.Jid(), htmltype, ui.str(), attrs)
}

func (ui *UiInputFloat) JawsUpdate(e *Element) {
	if f := ui.JawsGetFloat(e); ui.Last.Swap(f) != f {
		e.SetValue(ui.str())
	}
}

func (ui *UiInputFloat) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		err = ui.maybeDirty(ui, v, e, ui.FloatSetter.JawsSetFloat(e, v))
	}
	return
}
