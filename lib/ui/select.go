package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/named"
)

// Select renders an HTML select element backed by a [named.SelectHandler].
type Select struct {
	ContainerHelper
}

// NewSelect returns a select widget backed by sh.
func NewSelect(sh named.SelectHandler) *Select {
	return &Select{ContainerHelper: NewContainerHelper(sh)}
}

// JawsRender renders ui as an HTML select element.
func (ui *Select) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "select", params)
}

// JawsUpdate updates the selected value and child options.
func (ui *Select) JawsUpdate(e *jaws.Element) {
	// jawsbind.Setter[T] includes jawsbind.Getter[T]
	e.SetValue(ui.ContainerHelper.Container.(bind.Setter[string]).JawsGet(e))
	ui.UpdateContainer(e)
}

// JawsInput stores a browser-side select value.
func (ui *Select) JawsInput(e *jaws.Element, val string) (err error) {
	err = applyDirty(ui.Tag, e, ui.ContainerHelper.Container.(bind.Setter[string]).JawsSet(e, val))
	return
}

// Select renders an HTML select element.
func (rw RequestWriter) Select(sh named.SelectHandler, params ...any) error {
	return rw.UI(NewSelect(sh), params...)
}
