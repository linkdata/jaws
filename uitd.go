package jaws

import (
	"html/template"
	"io"
)

type UiTd struct {
	UiHtmlInner
}

func (ui *UiTd) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtmlInner.WriteHtmlInner(rq, w, "td", "", jid, data...)
}

func (rq *Request) Td(tagstring, inner string, attrs ...interface{}) template.HTML {
	ui := &UiTd{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: StringTags(tagstring)},
			HtmlInner: inner,
		},
	}
	return rq.UI(ui, attrs...)
}
