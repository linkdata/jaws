package ui

import (
	"html/template"
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
	"github.com/linkdata/jaws/core/htmlx"
)

type Textarea struct{ InputText }

func NewTextarea(g bind.Setter[string]) *Textarea { return &Textarea{InputText{Setter: g}} }
func (rw RequestWriter) Textarea(value any, params ...any) error {
	return rw.UI(NewTextarea(bind.MakeSetter[string](value)), params...)
}

func (ui *Textarea) JawsRender(e *core.Element, w io.Writer, params []any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		value := template.HTMLEscapeString(ui.JawsGet(e))
		err = htmlx.WriteHTMLInner(w, e.Jid(), "textarea", "", template.HTML(value), attrs...) // #nosec G203
	}
	return
}
func (ui *Textarea) JawsUpdate(e *core.Element) { e.SetValue(ui.JawsGet(e)) }
