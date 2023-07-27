package jaws

import (
	"html/template"
	"io"
)

type UiLi struct {
	UiClickable
}

func (ui *UiLi) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiClickable.WriteHtmlInner(rq, w, "li", "", jid, data...)
}

func (rq *Request) Li(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiLi{
		UiClickable{
			UiBase:    UiBase{Tags: StringTags(tagstring)},
			HtmlInner: inner,
			ClickFn:   fn,
		},
	}
	return rq.UI(ui, attrs...)
}
