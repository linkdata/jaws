package jaws

import (
	"io"
)

const ISO8601 = "2006-01-02"

type UiDate struct {
	UiInputDate
}

func (ui *UiDate) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderDateInput(e, w, e.Jid(), "date", params...)
}

func NewUiDate(g TimeSetter) *UiDate {
	return &UiDate{
		UiInputDate{
			TimeSetter: g,
		},
	}
}

func (rq *Request) Date(value interface{}, params ...interface{}) error {
	return rq.UI(NewUiDate(makeTimeSetter(value)), params...)
}
