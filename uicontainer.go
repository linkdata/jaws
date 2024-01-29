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

func (ui *UiContainer) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderContainer(e, w, ui.OuterHtmlTag, params)
}

func (rq RequestWriter) Container(outerHtmlTag string, c Container, params ...any) error {
	return rq.UI(NewUiContainer(outerHtmlTag, c), params...)
}
