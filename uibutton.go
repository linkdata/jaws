package jaws

import (
	"io"
)

type UiButton struct {
	UiHtmlInner
}

func (ui *UiButton) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "button", "button", params)
}

func NewUiButton(innerHtml HtmlGetter) *UiButton {
	return &UiButton{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) Button(innerHtml any, params ...any) error {
	return rq.UI(NewUiButton(MakeHtmlGetter(innerHtml)), params...)
}
