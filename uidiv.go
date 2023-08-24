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

func NewUiDiv(tags []interface{}, inner InnerProxy) *UiDiv {
	return &UiDiv{
		NewUiHtmlInner(tags, inner),
	}
}

func (rq *Request) Div(tagitem interface{}, inner interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiDiv(ProcessTags(tagitem), MakeInnerProxy(inner)), attrs...)
}
