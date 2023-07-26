package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiClickable struct {
	UiBase
	HtmlTag  string
	HtmlType string
	Text     string
	EventFn  ClickFn
}

func (ui *UiClickable) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return WriteHtmlInner(w, jid, ui.HtmlTag, ui.HtmlType, ui.Text, attrs...)
}

func (ui *UiClickable) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Click && ui.EventFn != nil {
		err = ui.EventFn(rq, jid)
	}
	return
}
