package jaws

import (
	"html/template"
	"io"
)

type UiNumber struct {
	UiInputFloat
}

func (ui *UiNumber) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderFloatInput(e, w, "number", params...)
}

func NewUiNumber(g FloatGetter) *UiNumber {
	return &UiNumber{
		UiInputFloat{
			FloatGetter: g,
		},
	}
}

func (rq *Request) Number(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiNumber(makeFloatGetter(value)), params...)
}
