package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiInput
	StringSetter
}

func (ui *UiInputText) renderStringInput(e *Element, w io.Writer, htmltype string, params ...any) error {
	ui.applyGetter(e, ui.StringSetter)
	attrs := e.ApplyParams(params)
	v := ui.JawsGetString(e)
	ui.Last.Store(v)
	return WriteHtmlInput(w, e.Jid(), htmltype, v, attrs)
}

func (ui *UiInputText) JawsUpdate(e *Element) {
	if v := ui.JawsGetString(e); ui.Last.Swap(v) != v {
		e.SetValue(v)
	}
}

func (ui *UiInputText) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		err = ui.maybeDirty(val, e, ui.StringSetter.JawsSetString(e, val))
	}
	return
}
