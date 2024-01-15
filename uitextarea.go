package jaws

import (
	"html/template"
	"io"
)

type UiTextarea struct {
	UiInputText
}

func (ui *UiTextarea) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	ui.parseGetter(e, ui.StringSetter)
	attrs := e.ParseParams(params)
	return WriteHtmlInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGetString(e)), attrs...) // #nosec G203
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

func (rq RequestWriter) Textarea(value interface{}, params ...interface{}) error {
	return rq.UI(NewUiTextarea(makeStringSetter(value)), params...)
}
