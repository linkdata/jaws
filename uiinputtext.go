package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiInput
	Setter[string]
}

func (ui *UiInputText) renderStringInput(e *Element, w io.Writer, htmltype string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		err = WriteHTMLInput(w, e.Jid(), htmltype, v, attrs)
	}
	return
}

func (ui *UiInputText) JawsUpdate(e *Element) {
	if v := ui.JawsGet(e); ui.Last.Swap(v) != v {
		e.SetValue(v)
	}
}

func (ui *UiInputText) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		err = ui.maybeDirty(val, e, ui.Setter.JawsSet(e, val))
	}
	return
}
