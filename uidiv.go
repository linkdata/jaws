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

func MakeUiDiv(innerHtml HtmlGetter) UiDiv {
	return UiDiv{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Div(innerHtml interface{}, params ...interface{}) template.HTML {
	ui := MakeUiDiv(makeHtmlGetter(innerHtml))
	return rq.UI(&ui, params...)
}
