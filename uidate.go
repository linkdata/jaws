package jaws

import (
	"html/template"
	"io"
)

const ISO8601 = "2006-01-02"

type UiDate struct {
	UiInputDate
}

func (ui *UiDate) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputDate.WriteHtmlInput(e, w, "date", e.Jid().String(), e.Data...)
}

func NewUiDate(tags []interface{}, vp ValueProxy) (ui *UiDate) {
	ui = &UiDate{
		UiInputDate: UiInputDate{
			UiInput: NewUiInput(tags, vp),
		},
	}
	return
}

func (rq *Request) Date(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiDate(ProcessTags(tagitem), MakeValueProxy(val)), attrs...)
}
