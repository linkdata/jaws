package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiHtml
	StringGetter
}

func (ui *UiInputText) renderStringInput(e *Element, w io.Writer, htmltype string, params ...interface{}) {
	if tagger, ok := ui.StringGetter.(TagGetter); ok {
		e.Tag(tagger.JawsGetTag(e))
	} else {
		e.Tag(ui.StringGetter)
	}
	attrs := ui.parseParams(e, params)
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, ui.JawsGetString(e), attrs...))
}

func (ui *UiInputText) JawsUpdate(u Updater) {
	u.SetValue(ui.JawsGetString(u.Element))
}

func (ui *UiInputText) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		err = ui.StringGetter.(StringSetter).JawsSetString(e, val)
		e.Jaws.Dirty(ui.StringGetter)
	}
	return
}
