package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/named"
	"github.com/linkdata/jaws/lib/what"
)

type Select struct {
	ContainerHelper
}

func NewSelect(sh named.SelectHandler) *Select {
	return &Select{ContainerHelper: NewContainerHelper(sh)}
}

func (rw RequestWriter) Select(sh named.SelectHandler, params ...any) error {
	return rw.UI(NewSelect(sh), params...)
}

func (ui *Select) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "select", params)
}

func (ui *Select) JawsUpdate(e *jaws.Element) {
	// jawsbind.Setter[T] includes jawsbind.Getter[T]
	e.SetValue(ui.ContainerHelper.Container.(bind.Setter[string]).JawsGet(e))
	ui.UpdateContainer(e)
}

func (ui *Select) JawsEvent(e *jaws.Element, wht what.What, val string) (err error) {
	err = jaws.ErrEventUnhandled
	if wht == what.Input {
		err = applyDirty(ui.Tag, e, ui.ContainerHelper.Container.(bind.Setter[string]).JawsSet(e, val))
	}
	return
}
