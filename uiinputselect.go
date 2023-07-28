package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputSelect struct {
	UiInput
	*NamedBoolArray
}

func (ui *UiInputSelect) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtml.WriteHtmlSelect(w, ui.NamedBoolArray, e.Jid, e.Data...)
}

func (ui *UiInputSelect) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid, val)
	}
	if wht == what.Input {
		ui.SetOnly(val)
	}
	return
}
