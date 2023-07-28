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

func NewUiDiv(tags []interface{}, inner string) *UiDiv {
	return &UiDiv{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) Div(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiDiv(ProcessTags(tagitem), inner), attrs...)
}
