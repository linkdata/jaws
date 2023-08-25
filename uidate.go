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

func NewUiDate(up Params) (ui *UiDate) {
	ui = &UiDate{
		UiInputDate: UiInputDate{
			UiInput: NewUiInput(up),
		},
	}
	return
}

func (rq *Request) Date(params ...interface{}) template.HTML {
	return rq.UI(NewUiDate(NewParams(params)), params...)
}
