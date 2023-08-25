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

func NewUiSpan(up Params) *UiSpan {
	return &UiSpan{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) Span(params ...interface{}) template.HTML {
	return rq.UI(NewUiSpan(NewParams(params)), params...)
}
