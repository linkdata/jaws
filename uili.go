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

func NewUiLi(innerHtml ValueProxy) *UiLi {
	return &UiLi{
		UiHtmlInner{
			UiValueProxy{
				ValueProxy: innerHtml,
			},
		},
	}
}

func (rq *Request) Li(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiLi(MakeValueProxy(innerHtml)), params...)
}
