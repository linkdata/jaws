package jaws

import (
	"html/template"
	"io"
)

type UiA struct {
	UiHtmlInner
}

func (ui *UiA) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "a", "", e.Data)
}

func NewUiA(up Params) *UiA {
	return &UiA{
		NewUiHtmlInner(up),
	}
}

func (rq *Request) A(params ...interface{}) template.HTML {
	return rq.UI(NewUiA(NewParams(params)), params...)
}
