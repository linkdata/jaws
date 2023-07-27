package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiHtml
	Value       string
	InputTextFn InputTextFn
}

func (ui *UiInputText) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	return ui.UiHtml.WriteHtmlInput(rq, w, htmltype, ui.Value, jid, data...)
}

func (ui *UiInputText) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input {
		old := ui.Value
		ui.Value = val
		if ui.InputTextFn != nil {
			if err = ui.InputTextFn(rq, jid, ui.Value); err != nil {
				ui.Value = old
			}
		}
	}
	return
}
