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
	ui.UiInputDate.WriteHtmlInput(e, w, e.Jid(), "date", params...)
}

func NewUiDate(vp ValueProxy) (ui *UiDate) {
	ui = &UiDate{
		UiInputDate{
			UiInput{
				UiValueProxy{
					ValueProxy: vp,
				},
			},
		},
	}
	return
}

func (rq *Request) Date(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiDate(MakeValueProxy(value)), params...)
}
