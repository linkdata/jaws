package jaws

import (
	"html"
	"html/template"
	"io"
)

type UiOption struct{ *NamedBool }

func (ui UiOption) JawsRender(e *Element, w io.Writer, params []any) error {
	e.Tag(ui.NamedBool)
	attrs := e.ApplyParams(params)
	valattr := template.HTMLAttr(`value="` + html.EscapeString(ui.JawsGetString(e)) + `"`) // #nosec G203
	attrs = append(attrs, valattr)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	return WriteHtmlInner(w, e.Jid(), "option", "", ui.JawsGetHtml(e), attrs...)
}

func (ui UiOption) JawsUpdate(e *Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
