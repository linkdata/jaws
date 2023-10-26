package jaws

import (
	"io"
)

type UiA struct {
	UiHtmlInner
}

func (ui *UiA) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderInner(e, w, "a", "", params)
}

func NewUiA(innerHtml HtmlGetter) *UiA {
	return &UiA{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) A(innerHtml interface{}, params ...interface{}) error {
	return rq.UI(NewUiA(makeHtmlGetter(innerHtml)), params...)
}
