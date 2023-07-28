package jaws

import (
	"html/template"
	"io"
)

type UiTd struct {
	UiHtmlInner
}

func (ui *UiTd) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "td", "")
}

func NewUiTd(tags []interface{}, inner string) *UiTd {
	return &UiTd{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) Td(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiTd(ProcessTags(tagitem), inner), attrs...)
}
