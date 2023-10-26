package jaws

import (
	"io"
)

type UiContainer struct {
	OuterHtmlTag string
	uiWrapContainer
}

func NewUiContainer(outerHtmlTag string, c Container) *UiContainer {
	return &UiContainer{
		OuterHtmlTag: outerHtmlTag,
		uiWrapContainer: uiWrapContainer{
			Container: c,
		},
	}
}

func (ui *UiContainer) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.renderContainer(e, w, ui.OuterHtmlTag, params)
}

func (rq *Request) Container(outerHtmlTag string, c Container, params ...interface{}) error {
	return rq.UI(NewUiContainer(outerHtmlTag, c), params...)
}
