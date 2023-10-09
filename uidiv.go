package jaws

import (
	"html/template"
	"io"
)

type UiDiv struct {
	UiHtmlInner
}

func (ui *UiDiv) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "div", "", params)
}

func NewUiDiv(innerHtml HtmlGetter) *UiDiv {
	return &UiDiv{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Div(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiDiv(makeHtmlGetter(innerHtml)), params...)
}
