package jaws

import (
	"io"
)

type UiSpan struct {
	UiHtmlInner
}

func (ui *UiSpan) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}

func NewUiSpan(innerHtml HtmlGetter) *UiSpan {
	return &UiSpan{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Span(innerHtml any, params ...any) error {
	return rq.UI(NewUiSpan(MakeHtmlGetter(innerHtml)), params...)
}
