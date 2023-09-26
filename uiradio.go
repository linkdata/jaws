package jaws

import (
	"html/template"
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderBoolInput(e, w, "radio", params...)
}

func NewUiRadio(vp BoolGetter) *UiRadio {
	return &UiRadio{
		UiInputBool{
			BoolGetter: vp,
		},
	}
}

func (rq *Request) Radio(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiRadio(makeBoolGetter(value)), params...)
}
