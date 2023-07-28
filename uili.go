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

func NewUiLi(tags []interface{}, inner string) *UiLi {
	return &UiLi{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) Li(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiLi(ProcessTags(tagitem), inner), attrs...)
}
