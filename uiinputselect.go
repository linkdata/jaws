package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputSelect struct {
	UiInput
	*NamedBoolArray
}

func (ui *UiInputSelect) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	attrs := ui.parseParams(e, params)
	ui.UiHtml.WriteHtmlSelect(w, e, ui.NamedBoolArray, attrs...)
}

func (ui *UiInputSelect) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		ui.UiInput.Set(e, val)
	}
	return
}
