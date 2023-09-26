package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputSelect struct {
	UiHtml
	*NamedBoolArray
}

func (ui *UiInputSelect) JawsRender(e *Element, w io.Writer, params []interface{}) {
	e.Tag(ui.NamedBoolArray)
	attrs := ui.parseParams(e, params)
	writeUiDebug(e, w)
	maybePanic(WriteHtmlSelect(w, e.Jid(), ui.NamedBoolArray, attrs...))
}

func (ui *UiInputSelect) JawsUpdate(u Updater) {
	u.SetValue(ui.NamedBoolArray.Get())
}

func (ui *UiInputSelect) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		ui.NamedBoolArray.Set(val, true)
		e.Jaws.Dirty(ui.NamedBoolArray)
	}
	return
}
