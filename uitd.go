package jaws

import (
	"html/template"
	"io"
)

type UiTd struct {
	UiClickable
}

func (ui *UiTd) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiClickable.WriteHtmlInner(rq, w, "td", "", jid, data...)
}

func (rq *Request) Td(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiTd{
		UiClickable{
			UiBase:    UiBase{Tags: StringTags(tagstring)},
			HtmlInner: inner,
			ClickFn:   fn,
		},
	}
	return rq.UI(ui, attrs...)
}
