package jaws

import (
	"io"
)

type UiText struct {
	UiInputText
}

func (ui *UiText) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderStringInput(e, w, "text", params...)
}

func NewUiText(vp StringSetter) (ui *UiText) {
	return &UiText{
		UiInputText{
			StringSetter: vp,
		},
	}
}

func (rq RequestWriter) Text(value interface{}, params ...interface{}) error {
	return rq.UI(NewUiText(makeStringSetter(value)), params...)
}
