package jaws

import (
	"html/template"
	"io"
)

const ISO8601 = "2006-01-02"

type UiDate struct {
	UiInputDate
}

func (ui *UiDate) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderDateInput(e, w, e.Jid(), "date", params...)
}

func MakeUiDate(g TimeGetter) UiDate {
	return UiDate{
		UiInputDate{
			TimeGetter: g,
		},
	}
}

func (rq *Request) Date(value interface{}, params ...interface{}) template.HTML {
	ui := MakeUiDate(makeTimeGetter(value))
	return rq.UI(&ui, params...)
}
