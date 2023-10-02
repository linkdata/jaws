package jaws

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws/what"
)

type UiTextArea struct {
	UiHtml
	StringGetter
}

func (ui *UiTextArea) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.parseGetter(e, ui.StringGetter)
	attrs := ui.parseParams(e, params)
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGetString(e)), attrs...))
}

func (ui *UiTextArea) JawsUpdate(e *Element) {
	e.SetInner(template.HTML(ui.JawsGetString(e)))
}

func (ui *UiTextArea) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		err = ui.StringGetter.(StringSetter).JawsSetString(e, val)
		e.Jaws.Dirty(ui.Tag)
	}
	return
}

func NewUiTextArea(g StringGetter) (ui *UiTextArea) {
	return &UiTextArea{
		StringGetter: g,
	}
}

func (rq *Request) TextArea(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTextArea(makeStringGetter(value)), params...)
}
