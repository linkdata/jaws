package jaws

import (
	"html/template"
	"io"
)

const ISO8601 = "2006-01-02"

type UiDate struct {
	UiInputDate
}

func (ui *UiDate) JawsRender(e *Element, w io.Writer) {
	ui.UiInputDate.WriteHtmlInput(e, w, e.Jid(), "date", e.Data...)
}

func NewUiDate(up Params) (ui *UiDate) {
	ui = &UiDate{
		UiInputDate: UiInputDate{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Date(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiDate(NewParams(value, params)), params...)
}
