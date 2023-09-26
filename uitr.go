package jaws

import (
	"html/template"
	"io"
)

type UiTr struct {
	UiHtmlInner
}

func (ui *UiTr) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "tr", "", params)
}

func MakeUiTr(innerHtml HtmlGetter) UiTr {
	return UiTr{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Tr(innerHtml interface{}, params ...interface{}) template.HTML {
	ui := MakeUiTr(makeHtmlGetter(innerHtml))
	return rq.UI(&ui, params...)
}
