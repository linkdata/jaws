package jaws

import (
	"io"
)

type UiTr struct {
	UiHtmlInner
}

func (ui *UiTr) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "tr", "", params)
}

func NewUiTr(innerHtml HtmlGetter) *UiTr {
	return &UiTr{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Tr(innerHtml interface{}, params ...interface{}) error {
	return rq.UI(NewUiTr(makeHtmlGetter(innerHtml)), params...)
}
