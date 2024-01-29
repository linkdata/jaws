package jaws

import (
	"io"
)

type UiRange struct {
	UiInputFloat
}

func (ui *UiRange) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderFloatInput(e, w, "range", params...)
}

func NewUiRange(g FloatSetter) *UiRange {
	return &UiRange{
		UiInputFloat{
			FloatSetter: g,
		},
	}
}

func (rq RequestWriter) Range(value any, params ...any) error {
	return rq.UI(NewUiRange(makeFloatSetter(value)), params...)
}
