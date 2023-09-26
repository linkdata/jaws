package jaws

import (
	"html/template"
	"io"
)

type UiTd struct {
	UiHtmlInner
}

func (ui *UiTd) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "td", "", params)
}

func NewUiTd(innerHtml HtmlGetter) *UiTd {
	return &UiTd{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Td(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTd(makeHtmlGetter(innerHtml)), params...)
}
