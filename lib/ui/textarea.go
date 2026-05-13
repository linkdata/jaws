package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Textarea renders an HTML textarea bound to a string setter.
type Textarea struct{ InputText }

// NewTextarea returns a textarea widget bound to g.
func NewTextarea(g bind.Setter[string]) *Textarea { return &Textarea{InputText{Setter: g}} }

// JawsRender renders ui as an HTML textarea.
func (u *Textarea) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if getterAttrs, err = u.applyGetterAttrs(elem, u.Setter); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		v := u.JawsGet(elem)
		u.Last.Store(v)
		v = template.HTMLEscapeString(v)
		err = htmlio.WriteHTMLInner(w, elem.Jid(), "textarea", "", template.HTML(v), attrs...) // #nosec G203
	}
	return
}

// Textarea renders an HTML textarea.
func (rw RequestWriter) Textarea(value any, params ...any) error {
	return rw.UI(NewTextarea(bind.MakeSetter[string](value)), params...)
}
