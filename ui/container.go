package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Container struct {
	OuterHTMLTag string
	WrapContainer
}

func NewContainer(outerHTMLTag string, c core.Container) *Container {
	return &Container{
		OuterHTMLTag:  outerHTMLTag,
		WrapContainer: NewWrapContainer(c),
	}
}

func (ui *Container) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, ui.OuterHTMLTag, params)
}

func (ui *Container) JawsUpdate(e *core.Element) {
	ui.UpdateContainer(e)
}
