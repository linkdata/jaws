package jaws

import (
	"html/template"
	"io"
)

type UiTextarea struct {
	UiInputText
}

func (ui *UiTextarea) JawsRender(e *Element, w io.Writer, params []any) error {
	ui.applyGetter(e, ui.Setter)
	attrs := e.ApplyParams(params)
	return WriteHTMLInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGet(e)), attrs...) // #nosec G203
}

func (ui *UiTextarea) JawsUpdate(e *Element) {
	e.SetValue(ui.JawsGet(e))
}

func NewUiTextarea(g Setter[string]) (ui *UiTextarea) {
	return &UiTextarea{
		UiInputText{
			Setter: g,
		},
	}
}

func (rq RequestWriter) Textarea(value any, params ...any) error {
	return rq.UI(NewUiTextarea(makeSetter[string](value)), params...)
}
