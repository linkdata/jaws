package jaws

import (
	"html/template"
	"io"
)

type UiSpan struct {
	UiHtmlInner
}

func (ui *UiSpan) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "span", "", e.Data)
}

func NewUiSpan(tags []interface{}, inner InnerProxy) *UiSpan {
	return &UiSpan{
		NewUiHtmlInner(tags, inner),
	}
}

func (rq *Request) Span(tagitem interface{}, inner interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiSpan(ProcessTags(tagitem), MakeInnerProxy(inner)), attrs...)
}
