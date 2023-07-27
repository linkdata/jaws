package jaws

import (
	"html/template"
	"io"
)

type UiButton struct {
	UiHtmlInner
}

func (ui *UiButton) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiHtmlInner.WriteHtmlInner(rq, w, "button", "button", jid, data...)
}

func (rq *Request) Button(tagstring, inner string, attrs ...interface{}) template.HTML {
	ui := &UiButton{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: StringTags(tagstring)},
			HtmlInner: inner,
		},
	}
	return rq.UI(ui, attrs...)
}
