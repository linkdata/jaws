package ui

import (
	"html"
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbool"
	"github.com/linkdata/jaws/jawshtml"
)

type Option struct{ *jawsbool.NamedBool }

func NewOption(nb *jawsbool.NamedBool) Option { return Option{NamedBool: nb} }
func (ui Option) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	e.Tag(ui.NamedBool)
	attrs := e.ApplyParams(params)
	valAttr := template.HTMLAttr(`value="` + html.EscapeString(ui.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valAttr)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	return jawshtml.WriteHTMLInner(w, e.Jid(), "option", "", ui.JawsGetHTML(e), attrs...)
}
func (ui Option) JawsUpdate(e *jaws.Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
