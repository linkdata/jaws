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

func NewUiTd(innerHtml Getter) *UiTd {
	return &UiTd{
		UiHtmlInner{
			UiGetter{
				Getter: innerHtml,
			},
		},
	}
}

func (rq *Request) Td(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTd(MakeGetter(innerHtml)), params...)
}
