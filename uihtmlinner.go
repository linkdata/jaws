package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	HtmlGetter
}

func (ui *UiHtmlInner) renderInner(e *Element, w io.Writer, htmltag, htmltype string, params []any) error {
	ui.applyGetter(e, ui.HtmlGetter)
	return WriteHtmlInner(w, e.Jid(), htmltag, htmltype, ui.JawsGetHtml(e), e.ApplyParams(params)...)
}

func (ui *UiHtmlInner) JawsUpdate(e *Element) {
	e.SetInner(ui.JawsGetHtml(e))
}
