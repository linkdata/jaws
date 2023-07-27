package jaws

import (
	"html/template"
	"io"
)

type UiA struct {
	UiHtmlInner
}

func (ui *UiA) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtmlInner.WriteHtmlInner(rq, w, "a", "", jid, data...)
}

func (rq *Request) A(tagstring, inner string, attrs ...interface{}) template.HTML {
	ui := &UiA{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: StringTags(tagstring)},
			HtmlInner: inner,
		},
	}
	return rq.UI(ui, attrs...)
}
