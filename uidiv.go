package jaws

import (
	"io"
)

type UiDiv struct {
	UiHtmlInner
}

func (ui *UiDiv) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}

func NewUiDiv(innerHtml HtmlGetter) *UiDiv {
	return &UiDiv{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Div(innerHtml any, params ...any) error {
	return rq.UI(NewUiDiv(makeHtmlGetter(innerHtml)), params...)
}
