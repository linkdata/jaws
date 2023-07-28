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

func NewUiTd(tags []interface{}, inner string) *UiTd {
	return &UiTd{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) Td(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiTd(ProcessTags(tagitem), inner), attrs...)
}
