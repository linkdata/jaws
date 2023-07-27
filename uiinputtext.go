package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiInput
}

func (ui *UiInputText) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	if val, ok := ui.Get().(string); ok {
		return ui.UiInput.WriteHtmlInput(rq, w, htmltype, val, jid, data...)
	}
	panic("jaws: UiInputText: not string")
}

func (ui *UiInputText) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(rq, wht, jid, val)
	}
	if wht == what.Input {
		ui.Set(val)
	}
	return
}
