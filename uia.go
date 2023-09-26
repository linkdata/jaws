package jaws

import (
	"html/template"
	"io"
)

type UiA struct {
	UiHtmlInner
}

func (ui *UiA) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "a", "", params)
}

func MakeUiA(innerHtml HtmlGetter) UiA {
	return UiA{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) A(innerHtml interface{}, params ...interface{}) template.HTML {
	ui := MakeUiA(makeHtmlGetter(innerHtml))
	return rq.UI(&ui, params...)
}
