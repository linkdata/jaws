package ui

import (
	"html"
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/named"
)

// Option renders an HTML option element backed by a [named.Bool].
type Option struct{ *named.Bool }

// NewOption returns an option widget backed by nb.
func NewOption(nb *named.Bool) Option { return Option{Bool: nb} }

// JawsRender renders ui as an HTML option element.
func (u Option) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	e.Tag(u.Bool)
	attrs := e.ApplyParams(params)
	valAttr := template.HTMLAttr(`value="` + html.EscapeString(u.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valAttr)
	if u.Checked() {
		attrs = append(attrs, "selected")
	}
	return htmlio.WriteHTMLInner(w, e.Jid(), "option", "", u.JawsGetHTML(e), attrs...)
}

// JawsUpdate updates the selected attribute.
func (u Option) JawsUpdate(e *jaws.Element) {
	if u.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
