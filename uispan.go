package jaws

import (
	"html/template"
	"io"
)

type UiSpan struct {
	UiHtmlInner
}

func (ui *UiSpan) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtmlInner.WriteHtmlInner(rq, w, "span", "", jid, data...)
}

func (rq *Request) Span(tagstring, inner string, attrs ...interface{}) template.HTML {
	ui := &UiSpan{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: StringTags(tagstring)},
			HtmlInner: inner,
		},
	}
	return rq.UI(ui, attrs...)
}
