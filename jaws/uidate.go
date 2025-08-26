package jaws

import (
	"io"
	"time"
)

const ISO8601 = "2006-01-02"

type UiDate struct {
	UiInputDate
}

func (ui *UiDate) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderDateInput(e, w, "date", params...)
}

func NewUiDate(g Setter[time.Time]) *UiDate {
	return &UiDate{
		UiInputDate{
			Setter: g,
		},
	}
}

func (rq RequestWriter) Date(value any, params ...any) error {
	return rq.UI(NewUiDate(makeSetter[time.Time](value)), params...)
}
