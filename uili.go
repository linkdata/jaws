package jaws

import (
	"html/template"
	"io"
)

type UiLi struct {
	UiHtmlInner
}

func (ui *UiLi) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "li", "", params)
}

func NewUiLi(innerHtml HtmlGetter) *UiLi {
	return &UiLi{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Li(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiLi(makeHtmlGetter(innerHtml)), params...)
}
