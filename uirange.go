package jaws

import (
	"html/template"
	"io"
)

type UiRange struct {
	UiInputFloat
}

func (ui *UiRange) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderFloatInput(e, w, "range", params...)
}

func NewUiRange(g FloatGetter) *UiRange {
	return &UiRange{
		UiInputFloat{
			FloatGetter: g,
		},
	}
}

func (rq *Request) Range(value interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiRange(makeFloatGetter(value)), params...)
}
