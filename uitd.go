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

func NewUiTd(tags []interface{}, inner InnerProxy) *UiTd {
	return &UiTd{
		NewUiHtmlInner(tags, inner),
	}
}

func (rq *Request) Td(tagitem interface{}, inner interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiTd(ProcessTags(tagitem), MakeInnerProxy(inner)), attrs...)
}
