package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiClickable struct {
	UiBase
	HtmlTag   string
	HtmlType  string
	HtmlInner string
	ClickFn   ClickFn
}

func (ui *UiClickable) WriteHtmlInner(rq *Request, w io.Writer, htmltag, htmltype, jid string, data ...interface{}) error {
	return ui.UiBase.WriteHtmlInner(rq, w, htmltag, htmltype, ui.HtmlInner, jid, data...)
}

func (ui *UiClickable) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return WriteHtmlInner(w, jid, ui.HtmlTag, ui.HtmlType, ui.HtmlInner, attrs...)
}

func (ui *UiClickable) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Click && ui.ClickFn != nil {
		err = ui.ClickFn(rq, jid)
	}
	return
}
