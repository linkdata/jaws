package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Container struct {
	OuterHTMLTag string
	WrapContainer
}

func NewContainer(outerHTMLTag string, c pkg.Container) *Container {
	return &Container{
		OuterHTMLTag:  outerHTMLTag,
		WrapContainer: NewWrapContainer(c),
	}
}

func (ui *Container) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, ui.OuterHTMLTag, params)
}

func (ui *Container) JawsUpdate(e *pkg.Element) {
	ui.UpdateContainer(e)
}
