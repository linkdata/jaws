package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	HtmlInner string
}

func (ui *UiHtmlInner) WriteHtmlInner(rq *Request, w io.Writer, htmltag, htmltype, jid string, data ...interface{}) error {
	return ui.UiHtml.WriteHtmlInner(rq, w, htmltag, htmltype, ui.HtmlInner, jid, data...)
}
