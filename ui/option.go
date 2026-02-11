package ui

import (
	"html"
	"html/template"
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Option struct{ *pkg.NamedBool }

func NewOption(nb *pkg.NamedBool) Option { return Option{NamedBool: nb} }
func (ui Option) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	e.Tag(ui.NamedBool)
	attrs := e.ApplyParams(params)
	valAttr := template.HTMLAttr(`value="` + html.EscapeString(ui.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valAttr)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	return pkg.WriteHTMLInner(w, e.Jid(), "option", "", ui.JawsGetHTML(e), attrs...)
}
func (ui Option) JawsUpdate(e *pkg.Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
