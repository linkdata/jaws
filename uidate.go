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

func (rq *Request) Date(tagstring string, val interface{}, attrs ...interface{}) template.HTML {
	ui := &UiDate{
		UiInputDate: UiInputDate{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: StringTags(tagstring)},
			},
		},
	}
	if t, ok := val.(time.Time); ok && t.IsZero() {
		val = time.Now()
	}
	ui.ProcessValue(val)
	return rq.UI(ui, attrs...)
}
