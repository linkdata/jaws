package jaws

import (
	"html"
	"html/template"
	"io"
)

type UiOption struct{ *NamedBool }

func (ui UiOption) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	e.Tag(ui.NamedBool)
	attrs := e.ParseParams(params)
	attrs = append(attrs, template.HTMLAttr(`value="`+html.EscapeString(ui.JawsGetString(e))+`"`))
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
