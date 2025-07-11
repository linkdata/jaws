package jaws

import (
	"html/template"
	"io"
)

type UiTextarea struct {
	UiInputText
}

func (ui *UiTextarea) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		err = WriteHTMLInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGet(e)), attrs...) // #nosec G203
	}
	return
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
