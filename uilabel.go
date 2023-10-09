package jaws

import (
	"html/template"
	"io"
)

type UiLabel struct {
	UiHtmlInner
}

func (ui *UiLabel) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderInner(e, w, "label", "", params)
}

func NewUiLabel(innerHtml HtmlGetter) *UiLabel {
	return &UiLabel{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Label(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiLabel(makeHtmlGetter(innerHtml)), params...)
}
