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

func MakeUiRadio(vp BoolGetter) UiRadio {
	return UiRadio{
		UiInputBool{
			BoolGetter: vp,
		},
	}
}

func (rq *Request) Radio(value interface{}, params ...interface{}) template.HTML {
	ui := MakeUiRadio(makeBoolGetter(value))
	return rq.UI(&ui, params...)
}
