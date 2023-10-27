package jaws

import (
	"io"
)

type UiDiv struct {
	UiHtmlInner
}

func (ui *UiDiv) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderInner(e, w, "div", "", params)
}

func NewUiDiv(innerHtml HtmlGetter) *UiDiv {
	return &UiDiv{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Div(innerHtml interface{}, params ...interface{}) error {
	return rq.UI(NewUiDiv(makeHtmlGetter(innerHtml)), params...)
}
