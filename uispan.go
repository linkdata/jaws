package jaws

import (
	"io"
)

type UiSpan struct {
	UiHtmlInner
}

func (ui *UiSpan) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "span", "", params)
}

func NewUiSpan(innerHtml HtmlGetter) *UiSpan {
	return &UiSpan{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Span(innerHtml interface{}, params ...interface{}) error {
	return rq.UI(NewUiSpan(makeHtmlGetter(innerHtml)), params...)
}
