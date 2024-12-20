package jaws

import (
	"io"
)

type UiText struct {
	UiInputText
}

func (ui *UiText) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "text", params...)
}

func NewUiText(vp Setter[string]) (ui *UiText) {
	return &UiText{
		UiInputText{
			Setter: vp,
		},
	}
}

func (rq RequestWriter) Text(value any, params ...any) error {
	return rq.UI(NewUiText(makeSetter[string](value)), params...)
}
