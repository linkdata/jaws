package jaws

import (
	"html/template"
	"io"
)

type UiDiv struct {
	UiClickable
}

func (ui *UiDiv) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiClickable.WriteHtmlInner(rq, w, "div", "", jid, data...)
}

func (rq *Request) Div(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiDiv{
		UiClickable{
			UiBase:    UiBase{Tags: StringTags(tagstring)},
			HtmlInner: inner,
			ClickFn:   fn,
		},
	}
	return rq.UI(ui, attrs...)
}
