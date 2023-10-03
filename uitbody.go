package jaws

import (
	"html/template"
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

func (ui *UiTbody) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderContainer(e, w, "tbody", params)
}

func (rq *Request) Tbody(c Container, params ...interface{}) template.HTML {
	return rq.UI(NewUiTbody(c), params...)
}
