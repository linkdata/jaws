package jaws

import (
	"html/template"
	"io"
)

type UiButton struct {
	UiClickable
}

func (ui *UiButton) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiClickable.WriteHtmlInner(rq, w, "button", "button", jid, data...)
}

func (rq *Request) Button(tagstring, inner string, fn ClickFn, attrs ...interface{}) template.HTML {
	ui := &UiButton{
		UiClickable{
			UiBase:    UiBase{Tags: StringTags(tagstring)},
			HtmlInner: inner,
			ClickFn:   fn,
		},
	}
	return rq.UI(ui, attrs...)
}
