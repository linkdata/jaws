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

func NewUiButton(tags []interface{}, inner string) *UiButton {
	return &UiButton{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) Button(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiButton(ProcessTags(tagitem), inner), attrs...)
}
