package jaws

import (
	"html/template"
	"io"
)

type UiA struct {
	UiHtmlInner
}

func (ui *UiA) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "a", "", params...)
}

func NewUiA(innerHtml ValueProxy) *UiA {
	return &UiA{
		NewUiHtmlInner(innerHtml),
	}
}

func (rq *Request) A(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiA(MakeValueProxy(innerHtml)), params...)
}
