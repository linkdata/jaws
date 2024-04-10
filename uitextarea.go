package jaws

import (
	"html/template"
	"io"
)

type UiTextarea struct {
	UiInputText
}

func (ui *UiTextarea) JawsRender(e *Element, w io.Writer, params []any) error {
	ui.parseGetter(e, ui.StringSetter)
	attrs := e.ApplyParams(params)
	return WriteHtmlInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGetString(e)), attrs...) // #nosec G203
}

func (ui *UiTextarea) JawsUpdate(e *Element) {
	e.SetValue(ui.JawsGetString(e))
}

func NewUiTextarea(g StringSetter) (ui *UiTextarea) {
	return &UiTextarea{
		UiInputText{
			StringSetter: g,
		},
	}
}

func (rq RequestWriter) Textarea(value any, params ...any) error {
	return rq.UI(NewUiTextarea(makeStringSetter(value)), params...)
}
