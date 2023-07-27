package jaws

import (
	"html/template"
	"io"
)

type UiSpan struct {
	UiClickable
}

func (ui *UiSpan) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiClickable.WriteHtmlInner(rq, w, "span", "", jid, data...)
}

func (rq *Request) Span(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiSpan{
		UiClickable{
			UiBase:    UiBase{Tags: StringTags(tagstring)},
			HtmlInner: inner,
			ClickFn:   fn,
		},
	}
	return rq.UI(ui, attrs...)
}
