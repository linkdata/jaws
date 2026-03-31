package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
	"github.com/linkdata/jaws/jawshtml"
)

type Textarea struct{ InputText }

func NewTextarea(g jawsbind.Setter[string]) *Textarea { return &Textarea{InputText{Setter: g}} }
func (rw RequestWriter) Textarea(value any, params ...any) error {
	return rw.UI(NewTextarea(jawsbind.MakeSetter[string](value)), params...)
}

func (ui *Textarea) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		value := template.HTMLEscapeString(ui.JawsGet(e))
		err = jawshtml.WriteHTMLInner(w, e.Jid(), "textarea", "", template.HTML(value), attrs...) // #nosec G203
	}
	return
}
func (ui *Textarea) JawsUpdate(e *jaws.Element) { e.SetValue(ui.JawsGet(e)) }
