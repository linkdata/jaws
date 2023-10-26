package jaws

import (
	"io"
)

type UiTbody struct {
	uiWrapContainer
}

func NewUiTbody(c Container) *UiTbody {
	return &UiTbody{
		uiWrapContainer{
			Container: c,
		},
	}
}

func (ui *UiTbody) JawsRender(e *Element, w io.Writer, params []interface{}) error {
	return ui.renderContainer(e, w, "tbody", params)
}

func (rq *Request) Tbody(c Container, params ...interface{}) error {
	return rq.UI(NewUiTbody(c), params...)
}
