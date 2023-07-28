package jaws

import (
	"html/template"
	"io"
)

type UiLi struct {
	UiHtmlInner
}

func (ui *UiLi) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "li", "")
}

func NewUiLi(tags []interface{}, inner string) *UiLi {
	return &UiLi{
		UiHtmlInner{
			UiHtml:    UiHtml{Tags: tags},
			HtmlInner: inner,
		},
	}
}

func (rq *Request) Li(tagitem interface{}, inner string, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiLi(ProcessTags(tagitem), inner), attrs...)
}
