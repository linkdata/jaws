package jaws

import (
	"html/template"
	"io"
)

type UiButton struct {
	UiHtmlInner
}

func (ui *UiButton) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "button", "button", params)
}

func NewUiButton(innerHtml HtmlGetter) *UiButton {
	return &UiButton{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Button(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiButton(makeHtmlGetter(innerHtml)), params...)
}
