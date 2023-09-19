package jaws

import (
	"html/template"
	"io"
)

type UiLi struct {
	UiHtmlInner
}

func (ui *UiLi) JawsRender(e *Element, w io.Writer, params ...interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "li", "", params...)
}

func NewUiLi(vp ValueProxy) *UiLi {
	return &UiLi{
		NewUiHtmlInner(vp),
	}
}

func (rq *Request) Li(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiLi(MakeValueProxy(innerHtml)), params...)
}
