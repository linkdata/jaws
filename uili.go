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

func NewUiLi(tags []interface{}, inner InnerProxy) *UiLi {
	return &UiLi{
		NewUiHtmlInner(tags, inner),
	}
}

func (rq *Request) Li(tagitem interface{}, inner interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiLi(ProcessTags(tagitem), MakeInnerProxy(inner)), attrs...)
}
