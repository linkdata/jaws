package jaws

import (
	"html/template"
	"io"
)

type UiLi struct {
	UiHtmlInner
}

func (ui *UiLi) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtmlInner.WriteHtmlInner(rq, w, "li", "", jid, data...)
}

func (rq *Request) Li(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiLi{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: StringTags(tagstring)},
			HtmlInner: inner,
			ClickFn:   fn,
		},
	}
	return rq.UI(ui, attrs...)
}
