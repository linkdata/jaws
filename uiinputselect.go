package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputSelect struct {
	UiHtml
	*NamedBoolArray
	InputTextFn InputTextFn
}

func (ui *UiInputSelect) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtml.WriteHtmlSelect(rq, w, ui.NamedBoolArray, jid, data...)
}

func (ui *UiInputSelect) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input {
		old := ui.Get()
		ui.SetOnly(val)
		if ui.InputTextFn != nil {
			if err = ui.InputTextFn(rq, jid, val); err != nil {
				ui.SetOnly(old)
			}
		}
	}
	return
}
