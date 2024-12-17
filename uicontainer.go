package jaws

import (
	"io"
)

type UiContainer struct {
	OuterHTMLTag string
	uiWrapContainer
}

func NewUiContainer(outerHTMLTag string, c Container) *UiContainer {
	return &UiContainer{
		OuterHTMLTag: outerHTMLTag,
		uiWrapContainer: uiWrapContainer{
			Container: c,
		},
	}
}

func (ui *UiContainer) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderContainer(e, w, ui.OuterHTMLTag, params)
}

func (rq RequestWriter) Container(outerHTMLTag string, c Container, params ...any) error {
	return rq.UI(NewUiContainer(outerHTMLTag, c), params...)
}
