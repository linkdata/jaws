package jaws

import (
	"html/template"
	"io"
)

type UiButton struct {
	UiHtmlInner
}

func (ui *UiButton) JawsRender(e *Element, w io.Writer) error {
	return ui.UiHtmlInner.WriteHtmlInner(e, w, "button", "button", e.Data)
}

func NewUiButton(tags []interface{}, inner InnerProxy) *UiButton {
	return &UiButton{
		NewUiHtmlInner(tags, inner),
	}
}

func (rq *Request) Button(tagitem interface{}, inner interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiButton(ProcessTags(tagitem), MakeInnerProxy(inner)), attrs...)
}
