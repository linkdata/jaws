package ui

import (
	"io"

	"github.com/linkdata/jaws"
)

// Container renders an HTML element around a dynamic child collection.
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

// JawsRender renders ui as its configured container element.
func (u *Container) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.RenderContainer(elem, w, u.OuterHTMLTag, params)
}

// JawsUpdate updates the child collection.
func (u *Container) JawsUpdate(elem *jaws.Element) {
	u.UpdateContainer(elem)
}

// Container renders c inside outerHTMLTag.
func (rw RequestWriter) Container(outerHTMLTag string, c jaws.Container, params ...any) error {
	return rw.UI(NewContainer(outerHTMLTag, c), params...)
}
