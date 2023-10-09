package jaws

import (
	"html"
	"io"
)

type UiOption struct{ *NamedBool }

func (ui UiOption) JawsRender(e *Element, w io.Writer, params []interface{}) {
	e.Tag(ui.NamedBool)
	writeUiDebug(e, w)
	attrs := parseParams(e, params)
	attrs = append(attrs, `value="`+html.EscapeString(ui.JawsGetString(e))+`"`)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	maybePanic(WriteHtmlInner(w, e.Jid(), "option", "", ui.JawsGetHtml(e), attrs...))
}

func (ui UiOption) JawsUpdate(e *Element) {
	e.SetValue(ui.NamedBool.JawsGetString(e))
	e.SetInner(ui.NamedBool.JawsGetHtml(e))
}
