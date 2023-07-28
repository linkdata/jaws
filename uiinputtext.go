package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiInput
}

func (ui *UiInputText) WriteHtmlInput(e *Element, w io.Writer, htmltype string) error {
	if val, ok := ui.Get(e).(string); ok {
		return ui.UiInput.WriteHtmlInput(w, htmltype, val, e.Jid, e.Data...)
	}
	panic("jaws: UiInputText: not string")
}

func (ui *UiInputText) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid, val)
	}
	if wht == what.Input {
		err = ui.Set(e, val)
	}
	return
}
