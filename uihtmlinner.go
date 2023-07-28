package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	HtmlInner string
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string) error {
	return ui.UiHtml.WriteHtmlInner(w, htmltag, htmltype, ui.HtmlInner, e.Jid, e.Data...)
}
