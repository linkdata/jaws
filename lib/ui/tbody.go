package ui

import (
	"io"

	"github.com/linkdata/jaws"
)

// Tbody renders an HTML tbody containing dynamic child rows.
type Tbody struct {
	ContainerHelper
}

// NewTbody returns a tbody widget that renders and updates c as table rows.
func NewTbody(c jaws.Container) *Tbody {
	return &Tbody{ContainerHelper: NewContainerHelper(c)}
}

// JawsRender renders ui as an HTML tbody element.
func (ui *Tbody) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "tbody", params)
}

// JawsUpdate updates the child rows.
func (ui *Tbody) JawsUpdate(e *jaws.Element) {
	ui.UpdateContainer(e)
}

// Tbody renders an HTML tbody element.
func (rw RequestWriter) Tbody(c jaws.Container, params ...any) error {
	return rw.UI(NewTbody(c), params...)
}
