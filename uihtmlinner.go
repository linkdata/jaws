package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	HtmlGetter
}

func (ui *UiHtmlInner) renderInner(e *Element, w io.Writer, htmltag, htmltype string, params []interface{}) {
	ui.parseGetter(e, ui.HtmlGetter)
	attrs := ui.parseParams(e, params)
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInner(w, e.Jid(), htmltag, htmltype, ui.JawsGetHtml(e), attrs...))
}

func (ui *UiHtmlInner) JawsUpdate(u Updater) {
	u.SetInner(ui.JawsGetHtml(u.Element))
}
