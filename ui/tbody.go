package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Tbody struct {
	WrapContainer
}

func NewTbody(c pkg.Container) *Tbody {
	return &Tbody{WrapContainer: NewWrapContainer(c)}
}

func (ui *Tbody) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "tbody", params)
}

func (ui *Tbody) JawsUpdate(e *pkg.Element) {
	ui.UpdateContainer(e)
}
