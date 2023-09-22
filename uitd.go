package jaws

import (
	"html/template"
	"io"
)

type UiTd struct {
	UiHtmlInner
}

func (ui *UiTd) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "td", "", params...)
}

func NewUiTd(innerHtml ValueProxy) *UiTd {
	return &UiTd{
		UiHtmlInner{
			UiValueProxy{
				ValueProxy: innerHtml,
			},
		},
	}
}

func (rq *Request) Td(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTd(MakeValueProxy(innerHtml)), params...)
}
