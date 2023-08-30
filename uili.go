package jaws

import (
	"html/template"
	"io"
)

type UiLi struct {
	UiHtmlInner
}

func (ui *UiLi) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "li", "", e.Data)
}

func NewUiLi(up Params) *UiLi {
	return &UiLi{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) Li(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiLi(NewParams(innerHtml, params)), params...)
}
