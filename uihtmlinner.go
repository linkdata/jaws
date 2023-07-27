package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiHtmlInner struct {
	UiHtml
	HtmlInner string
	ClickFn   ClickFn
}

func (ui *UiHtmlInner) WriteHtmlInner(rq *Request, w io.Writer, htmltag, htmltype, jid string, data ...interface{}) error {
	return ui.UiHtml.WriteHtmlInner(rq, w, htmltag, htmltype, ui.HtmlInner, jid, data...)
}

func (ui *UiHtmlInner) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Click && ui.ClickFn != nil {
		err = ui.ClickFn(rq, jid)
	}
	return
}
