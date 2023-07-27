package jaws

import (
	"html/template"
	"io"
)

type UiA struct {
	UiClickable
}

func (ui *UiA) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiClickable.WriteHtmlInner(rq, w, "a", "", jid, data...)
}

func (rq *Request) A(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiA{
		UiClickable{
			UiBase:    UiBase{Tags: StringTags(tagstring)},
			HtmlInner: inner,
			ClickFn:   fn,
		},
	}
	return rq.UI(ui, attrs...)
}
