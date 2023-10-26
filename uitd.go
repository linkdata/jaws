package jaws

import (
	"io"
)

type UiTd struct {
	UiHtmlInner
}

func (ui *UiTd) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderInner(e, w, "td", "", params)
}

func NewUiTd(innerHtml HtmlGetter) *UiTd {
	return &UiTd{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Td(innerHtml interface{}, params ...interface{}) error {
	return rq.UI(NewUiTd(makeHtmlGetter(innerHtml)), params...)
}
