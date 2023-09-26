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

func MakeUiLabel(innerHtml HtmlGetter) UiLabel {
	return UiLabel{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq *Request) Label(innerHtml interface{}, params ...interface{}) template.HTML {
	ui := MakeUiLabel(makeHtmlGetter(innerHtml))
	return rq.UI(&ui, params...)
}
