package jaws

import (
	"html/template"
	"io"
)

type UiTd struct {
	UiHtmlInner
}

func (ui *UiTd) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "td", "", e.Data)
}

func NewUiTd(up Params) *UiTd {
	return &UiTd{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) Td(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiTd(NewParams(innerHtml, params)), params...)
}
