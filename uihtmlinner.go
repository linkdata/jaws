package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	HtmlGetter
}

func (ui *UiHtmlInner) renderInner(e *Element, w io.Writer, htmltag, htmltype string, params []interface{}) error {
	ui.parseGetter(e, ui.HtmlGetter)
	attrs := ui.parseParams(e, params)
	return WriteHtmlInner(w, e.Jid(), htmltag, htmltype, ui.JawsGetHtml(e), attrs...)
}

func (ui *UiHtmlInner) JawsUpdate(e *Element) {
	e.SetInner(ui.JawsGetHtml(e))
}
