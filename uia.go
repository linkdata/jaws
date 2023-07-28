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

func NewUiA(tags []interface{}, inner string) *UiA {
	return &UiA{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) A(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiA(ProcessTags(tagitem), inner), attrs...)
}
