package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
)

type Select struct {
	ContainerHelper
}

func NewSelect(sh core.SelectHandler) *Select {
	return &Select{ContainerHelper: NewContainerHelper(sh)}
}

func (rw RequestWriter) Select(sh core.SelectHandler, params ...any) error {
	return rw.UI(NewSelect(sh), params...)
}

func (ui *Select) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "select", params)
}

func (ui *Select) JawsUpdate(e *core.Element) {
	// core.Setter[T] includes core.Getter[T]
	e.SetValue(ui.ContainerHelper.Container.(core.Setter[string]).JawsGet(e))
	ui.UpdateContainer(e)
}

func (ui *Select) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		_, err = applyDirty(ui.Tag, e, ui.ContainerHelper.Container.(core.Setter[string]).JawsSet(e, val))
	}
	return
}
