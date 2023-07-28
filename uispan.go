package jaws

import (
	"html/template"
	"io"
)

type UiSpan struct {
	UiHtmlInner
}

func (ui *UiSpan) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "span", "")
}

func NewUiSpan(tags []interface{}, inner string) *UiSpan {
	return &UiSpan{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) Span(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiSpan(ProcessTags(tagitem), inner), attrs...)
}
