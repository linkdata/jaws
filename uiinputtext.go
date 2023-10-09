package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiInput
	StringGetter
}

func (ui *UiInputText) renderStringInput(e *Element, w io.Writer, htmltype string, params ...interface{}) {
	ui.parseGetter(e, ui.StringGetter)
	attrs := ui.parseParams(e, params)
	v := ui.JawsGetString(e)
	ui.Last.Store(v)
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, v, attrs...))
}

func (ui *UiInputText) JawsUpdate(e *Element) {
	if v := ui.JawsGetString(e); ui.Last.Swap(v) != v {
		e.SetValue(v)
	}
}

func (ui *UiInputText) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ui.UiHtml.JawsEvent(e, wht, val)
	if wht == what.Input {
		ui.Last.Store(val)
		if err == nil {
			err = ui.StringGetter.(StringSetter).JawsSetString(e, val)
		}
		e.Dirty(ui.Tag)
	}
	return
}
