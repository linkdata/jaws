package jaws

import (
	"html/template"
	"io"
)

type UiTr struct {
	UiHtmlInner
}

func (ui *UiTr) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "tr", "", e.Data)
}

func NewUiTr(up Params) *UiTr {
	return &UiTr{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) Tr(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTr(NewParams(innerHtml, params)), params...)
}
