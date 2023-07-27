package jaws

import (
	"html/template"
	"io"
)

type UiDiv struct {
	UiHtmlInner
}

func (ui *UiDiv) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtmlInner.WriteHtmlInner(rq, w, "div", "", jid, data...)
}

func (rq *Request) Div(tagstring, inner string, attrs ...interface{}) template.HTML {
	ui := &UiDiv{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: StringTags(tagstring)},
			HtmlInner: inner,
		},
	}
	return rq.UI(ui, attrs...)
}
