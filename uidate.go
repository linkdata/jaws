package jaws

import (
	"html/template"
	"io"
	"time"
)

const ISO8601 = "2006-01-02"

type UiDate struct {
	UiInputDate
}

func (ui *UiDate) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	return ui.UiInputDate.WriteHtmlInput(rq, w, "date", jid, data...)
}

func (rq *Request) Date(tagstring string, val time.Time, attrs ...interface{}) template.HTML {
	if val.IsZero() {
		val = time.Now()
	}
	ui := &UiDate{
		UiInputDate: UiInputDate{
			UiHtml: UiHtml{Tags: StringTags(tagstring)},
			Value:  val,
		},
	}
	return rq.UI(ui, attrs...)
}
