package ui

import (
	"html/template"
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Textarea struct{ InputText }

func NewTextarea(g pkg.Setter[string]) *Textarea { return &Textarea{InputText{Setter: g}} }
func (ui *Textarea) JawsRender(e *pkg.Element, w io.Writer, params []any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		err = pkg.WriteHTMLInner(w, e.Jid(), "textarea", "", template.HTML(ui.JawsGet(e)), attrs...) // #nosec G203
	}
	return
}
func (ui *Textarea) JawsUpdate(e *pkg.Element) { e.SetValue(ui.JawsGet(e)) }
