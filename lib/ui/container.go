package ui

import (
	"io"

	"github.com/linkdata/jaws"
)

type Container struct {
	OuterHTMLTag string
	ContainerHelper
}

func NewContainer(outerHTMLTag string, c jaws.Container) *Container {
	return &Container{
		OuterHTMLTag:    outerHTMLTag,
		ContainerHelper: NewContainerHelper(c),
	}
}

func (rw RequestWriter) Container(outerHTMLTag string, c jaws.Container, params ...any) error {
	return rw.UI(NewContainer(outerHTMLTag, c), params...)
}

func (ui *Container) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, ui.OuterHTMLTag, params)
}

func (ui *Container) JawsUpdate(e *jaws.Element) {
	ui.UpdateContainer(e)
}
