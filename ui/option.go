package ui

import (
	"html"
	"html/template"
	"io"

	"github.com/linkdata/jaws/core"
)

type Option struct{ *core.NamedBool }

func NewOption(nb *core.NamedBool) Option { return Option{NamedBool: nb} }
func (ui Option) JawsRender(e *core.Element, w io.Writer, params []any) error {
	e.Tag(ui.NamedBool)
	attrs := e.ApplyParams(params)
	valAttr := template.HTMLAttr(`value="` + html.EscapeString(ui.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valAttr)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	return core.WriteHTMLInner(w, e.Jid(), "option", "", ui.JawsGetHTML(e), attrs...)
}
func (ui Option) JawsUpdate(e *core.Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
