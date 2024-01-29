package jaws

import (
	"io"
)

type UiPassword struct {
	UiInputText
}

func (ui *UiPassword) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "password", params...)
}

func NewUiPassword(g StringSetter) *UiPassword {
	return &UiPassword{
		UiInputText{
			StringSetter: g,
		},
	}
}

func (rq RequestWriter) Password(value any, params ...any) error {
	return rq.UI(NewUiPassword(makeStringSetter(value)), params...)
}
