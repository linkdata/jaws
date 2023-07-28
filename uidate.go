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

func NewUiDate(tags []interface{}, val interface{}) (ui *UiDate) {
	ui = &UiDate{
		UiInputDate: UiInputDate{
			UiInput: UiInput{
				UiHtml: UiHtml{Tags: tags},
			},
		},
	}
	ui.ProcessValue(val)
	return
}

func (rq *Request) Date(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	if t, ok := val.(time.Time); ok && t.IsZero() {
		val = time.Now()
	}
	return rq.UI(NewUiDate(ProcessTags(tagitem), val), attrs...)
}
