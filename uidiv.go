package jaws

import (
	"html/template"
	"io"
)

type UiDiv struct {
	UiHtmlInner
}

func (ui *UiDiv) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "div", "", params...)
}

func NewUiDiv(innerHtml Getter) *UiDiv {
	return &UiDiv{
		UiHtmlInner{
			UiGetter{
				Getter: innerHtml,
			},
		},
	}
}

func (rq *Request) Div(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiDiv(MakeValueProxy(innerHtml)), params...)
}
