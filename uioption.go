package jaws

import (
	"html"
	"io"
)

type UiOption struct{ *NamedBool }

func (ui UiOption) JawsRender(e *Element, w io.Writer, params []interface{}) {
	e.Tag(ui.NamedBool)
	attrs := parseParams(e, params)
	attrs = append(attrs, `value="`+html.EscapeString(ui.JawsGetString(e))+`"`)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	maybePanic(WriteHtmlInner(w, e.Jid(), "option", "", ui.JawsGetHtml(e), attrs...))
}

func (ui UiOption) JawsUpdate(e *Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
