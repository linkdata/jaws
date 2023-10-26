package jaws

import (
	"io"
)

type UiLabel struct {
	UiHtmlInner
}

func (ui *UiLabel) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderInner(e, w, "label", "", params)
}

func NewUiLabel(innerHtml HtmlGetter) *UiLabel {
	return &UiLabel{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Label(innerHtml interface{}, params ...interface{}) error {
	return rq.UI(NewUiLabel(makeHtmlGetter(innerHtml)), params...)
}
