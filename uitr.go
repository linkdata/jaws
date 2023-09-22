package jaws

import (
	"html/template"
	"io"
)

type UiTr struct {
	UiHtmlInner
}

func (ui *UiTr) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "tr", "", params...)
}

func NewUiTr(innerHtml Getter) *UiTr {
	return &UiTr{
		UiHtmlInner{
			UiGetter{
				Getter: innerHtml,
			},
		},
	}
}

func (rq *Request) Tr(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTr(MakeValueProxy(innerHtml)), params...)
}
