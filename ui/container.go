package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Container struct {
	OuterHTMLTag string
	ContainerHelper
}

func NewContainer(outerHTMLTag string, c core.Container) *Container {
	return &Container{
		OuterHTMLTag:    outerHTMLTag,
		ContainerHelper: NewContainerHelper(c),
	}
}

func (rw RequestWriter) Container(outerHTMLTag string, c core.Container, params ...any) error {
	return rw.UI(NewContainer(outerHTMLTag, c), params...)
}

func (ui *Container) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, ui.OuterHTMLTag, params)
}

func (ui *Container) JawsUpdate(e *core.Element) {
	ui.UpdateContainer(e)
}
