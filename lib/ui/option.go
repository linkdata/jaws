package ui

import (
	"html"
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/named"
)

type Option struct{ *named.Bool }

// NewOption returns an option widget backed by nb.
func NewOption(nb *named.Bool) Option { return Option{Bool: nb} }

func (ui Option) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	e.Tag(ui.Bool)
	attrs := e.ApplyParams(params)
	valAttr := template.HTMLAttr(`value="` + html.EscapeString(ui.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valAttr)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	return htmlio.WriteHTMLInner(w, e.Jid(), "option", "", ui.JawsGetHTML(e), attrs...)
}
func (ui Option) JawsUpdate(e *jaws.Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
