package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Tbody struct {
	ContainerHelper
}

func NewTbody(c core.Container) *Tbody {
	return &Tbody{ContainerHelper: NewContainerHelper(c)}
}

func (rw RequestWriter) Tbody(c core.Container, params ...any) error {
	return rw.UI(NewTbody(c), params...)
}

func (ui *Tbody) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "tbody", params)
}

func (ui *Tbody) JawsUpdate(e *core.Element) {
	ui.UpdateContainer(e)
}
