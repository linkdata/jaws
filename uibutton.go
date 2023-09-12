package jaws

import (
	"html/template"
	"io"
)

type UiButton struct {
	UiHtmlInner
}

func (ui *UiButton) JawsRender(e *Element, w io.Writer) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "button", "button", e.Data)
}

func NewUiButton(up Params) *UiButton {
	return &UiButton{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) Button(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiButton(NewParams(innerHtml, params)), params...)
}
