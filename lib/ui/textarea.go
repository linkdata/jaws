package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

type Textarea struct{ InputText }

func NewTextarea(g bind.Setter[string]) *Textarea { return &Textarea{InputText{Setter: g}} }
func (rw RequestWriter) Textarea(value any, params ...any) error {
	return rw.UI(NewTextarea(bind.MakeSetter[string](value)), params...)
}

func (ui *Textarea) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if getterAttrs, err = ui.applyGetterAttrs(e, ui.Setter); err == nil {
		attrs := append(e.ApplyParams(params), getterAttrs...)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		v = template.HTMLEscapeString(v)
		err = htmlio.WriteHTMLInner(w, e.Jid(), "textarea", "", template.HTML(v), attrs...) // #nosec G203
	}
	return
}
