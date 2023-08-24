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
	writeUiDebug(e, w)
	return ui.UiHtml.WriteHtmlSelect(w, ui.NamedBoolArray, e.Jid().String(), e.Data...)
}

func (ui *UiInputSelect) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request(), wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		ui.SetOnly(val)
	}
	return
}
