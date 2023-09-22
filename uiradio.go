package jaws

import (
	"html/template"
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiInputBool.WriteHtmlInput(e, w, "radio", params...)
}

func NewUiRadio(vp ValueProxy) (ui *UiRadio) {
	ui = &UiRadio{
		UiInputBool{
			UiInput{
				UiValueProxy{
					ValueProxy: vp,
				},
			},
		},
	}
	return
}

func (rq *Request) Radio(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiRadio(MakeValueProxy(value)), params...)
}
