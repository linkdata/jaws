package ui

import (
	"html"
	"html/template"
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/htmlx"
	"github.com/linkdata/jaws/core/named"
)

type Option struct{ *named.NamedBool }

func NewOption(nb *named.NamedBool) Option { return Option{NamedBool: nb} }
func (ui Option) JawsRender(e *core.Element, w io.Writer, params []any) error {
	e.Tag(ui.NamedBool)
	attrs := e.ApplyParams(params)
	valAttr := template.HTMLAttr(`value="` + html.EscapeString(ui.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valAttr)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	return htmlx.WriteHTMLInner(w, e.Jid(), "option", "", ui.JawsGetHTML(e), attrs...)
}
func (ui Option) JawsUpdate(e *core.Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
