package jaws

import (
	"io"
)

type UiTd struct {
	UiHtmlInner
}

func (ui *UiTd) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}

func NewUiTd(innerHtml HtmlGetter) *UiTd {
	return &UiTd{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Td(innerHtml any, params ...any) error {
	return rq.UI(NewUiTd(MakeHtmlGetter(innerHtml)), params...)
}
