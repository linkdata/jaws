package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Textarea renders an HTML textarea bound to a string setter.
//
// A Textarea value must back at most one live [jaws.Element]. Construct distinct
// Textarea values over the same setter to render one bound value more than once.
//
// Each browser edit sends the complete value in one WebSocket message. The
// value plus protocol and JSON overhead must fit the 32 KiB inbound limit
// documented by [jaws.Request.ServeHTTP]. Use a conservative maxlength or a
// separate upload endpoint for potentially large text.
type Textarea struct{ InputText }

// NewTextarea returns a textarea widget bound to g.
//
// For writable use, g must provide the setter-derived dirty target described by [Input].
// See [Textarea] for the browser-to-server message-size limit.
func NewTextarea(g bind.Setter[string]) *Textarea { return &Textarea{InputText{Setter: g}} }

// JawsRender renders ui as an HTML textarea.
func (u *Textarea) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	getterAttrs := u.applyGetterAttrs(elem, u.Setter)
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	v := u.JawsGet(elem)
	u.Last.Store(v)
	v = template.HTMLEscapeString(v)
	err = htmlio.WriteHTMLInner(w, elem.Jid(), "textarea", "", template.HTML(v), attrs...) // #nosec G203
	return
}

// Textarea renders an HTML textarea.
//
// See [Textarea] for the browser-to-server message-size limit.
func (rw RequestWriter) Textarea(value any, params ...any) error {
	return rw.NewUI(NewTextarea(bind.MakeSetter[string](value)), params...)
}
