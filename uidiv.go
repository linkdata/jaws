package jaws

import (
	"html/template"
	"io"
)

type UiDiv struct {
	UiHtmlInner
}

func (ui *UiDiv) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "div", "", e.Data)
}

func NewUiDiv(up Params) *UiDiv {
	return &UiDiv{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) Div(params ...interface{}) template.HTML {
	return rq.UI(NewUiDiv(NewParams(params)), params...)
}
