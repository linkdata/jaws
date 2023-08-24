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

func NewUiA(tags []interface{}, inner InnerProxy) *UiA {
	return &UiA{
		NewUiHtmlInner(tags, inner),
	}
}

func (rq *Request) A(tagitem interface{}, inner interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiA(ProcessTags(tagitem), MakeInnerProxy(inner)), attrs...)
}
