package ui

import (
	"io"

	"github.com/linkdata/jaws"
)

type Container struct {
	OuterHTMLTag string
	ContainerHelper
}

// NewContainer returns a container widget that renders c inside outerHTMLTag.
// The returned widget tracks child elements and updates them using ContainerHelper.
func NewContainer(outerHTMLTag string, c jaws.Container) *Container {
	return &Container{
		OuterHTMLTag:    outerHTMLTag,
		ContainerHelper: NewContainerHelper(c),
	}
}

func (ui *Container) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, ui.OuterHTMLTag, params)
}

func (ui *Container) JawsUpdate(e *jaws.Element) {
	ui.UpdateContainer(e)
}

func (rw RequestWriter) Container(outerHTMLTag string, c jaws.Container, params ...any) error {
	return rw.UI(NewContainer(outerHTMLTag, c), params...)
}
