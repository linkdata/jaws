package jaws

import (
	"html/template"
	"io"
)

type UiTextarea struct {
	UiInputText
}

func (ui *UiTextarea) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.parseGetter(e, ui.StringSetter)
	attrs := ui.parseParams(e, params)
	maybePanic(WriteHtmlInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGetString(e)), attrs...)) // #nosec G203
}

func (ui *UiTextarea) JawsUpdate(e *Element) {
	e.SetInner(template.HTML(ui.JawsGetString(e))) // #nosec G203
}

func NewUiTextarea(g StringSetter) (ui *UiTextarea) {
	return &UiTextarea{
		UiInputText{
			StringSetter: g,
		},
	}
}

func (rq *Request) Textarea(value interface{}, params ...interface{}) error {
	return rq.UI(NewUiTextarea(makeStringSetter(value)), params...)
}
