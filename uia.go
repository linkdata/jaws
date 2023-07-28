package jaws

import (
	"html/template"
	"io"
)

type UiA struct {
	UiHtmlInner
}

func (ui *UiA) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "a", "")
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
