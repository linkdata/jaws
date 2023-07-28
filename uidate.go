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

func (ui *UiDate) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputDate.WriteHtmlInput(e, w, "date", e.Jid, e.Data...)
}

func NewUiDate(tags []interface{}, vp ValueProxy) (ui *UiDate) {
	ui = &UiDate{
		UiInputDate: UiInputDate{
			UiInput: UiInput{
				UiHtml:     UiHtml{Tags: tags},
				ValueProxy: vp,
			},
		},
	}
	return
}

func (rq *Request) Date(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	if t, ok := val.(time.Time); ok && t.IsZero() {
		val = time.Now()
	}
	return rq.UI(NewUiDate(ProcessTags(tagitem), MakeValueProxy(val)), attrs...)
}
