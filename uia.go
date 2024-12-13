package jaws

import (
	"io"
)

type UiA struct {
	UiHtmlInner
}

func (ui *UiA) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "a", "", params)
}

func NewUiA(innerHtml HtmlGetter) *UiA {
	return &UiA{
		UiHtmlInner{
			HtmlGetter: innerHtml,
		},
	}
}

func (rq RequestWriter) A(innerHtml any, params ...any) error {
	return rq.UI(NewUiA(MakeHtmlGetter(innerHtml)), params...)
}
