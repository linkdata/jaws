package jaws

import (
	"io"
)

type UiButton struct {
	UiHtmlInner
}

func (ui *UiButton) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderInner(e, w, "button", "button", params)
}

func NewUiButton(innerHtml HtmlGetter) *UiButton {
	return &UiButton{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Button(innerHtml interface{}, params ...interface{}) error {
	return rq.UI(NewUiButton(makeHtmlGetter(innerHtml)), params...)
}
