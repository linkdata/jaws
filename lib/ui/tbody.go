package ui

import (
	"io"

	"github.com/linkdata/jaws"
)

type Tbody struct {
	ContainerHelper
}

// NewTbody returns a tbody widget that renders and updates c as table rows.
func NewTbody(c jaws.Container) *Tbody {
	return &Tbody{ContainerHelper: NewContainerHelper(c)}
}

func (ui *Tbody) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "tbody", params)
}

func (ui *Tbody) JawsUpdate(e *jaws.Element) {
	ui.UpdateContainer(e)
}

func (rw RequestWriter) Tbody(c jaws.Container, params ...any) error {
	return rw.UI(NewTbody(c), params...)
}
