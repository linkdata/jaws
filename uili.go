package jaws

import (
	"io"
)

type UiLi struct {
	UiHtmlInner
}

func (ui *UiLi) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}

func NewUiLi(innerHtml HtmlGetter) *UiLi {
	return &UiLi{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Li(innerHtml any, params ...any) error {
	return rq.UI(NewUiLi(makeHtmlGetter(innerHtml)), params...)
}
