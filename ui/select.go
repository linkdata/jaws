package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
	"github.com/linkdata/jaws/core/jawsbool"
	"github.com/linkdata/jaws/what"
)

type Select struct {
	ContainerHelper
}

func NewSelect(sh jawsbool.SelectHandler) *Select {
	return &Select{ContainerHelper: NewContainerHelper(sh)}
}

func (rw RequestWriter) Select(sh jawsbool.SelectHandler, params ...any) error {
	return rw.UI(NewSelect(sh), params...)
}

func (ui *Select) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "select", params)
}

func (ui *Select) JawsUpdate(e *core.Element) {
	// jawsbind.Setter[T] includes jawsbind.Getter[T]
	e.SetValue(ui.ContainerHelper.Container.(jawsbind.Setter[string]).JawsGet(e))
	ui.UpdateContainer(e)
}

func (ui *Select) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		err = applyDirty(ui.Tag, e, ui.ContainerHelper.Container.(jawsbind.Setter[string]).JawsSet(e, val))
	}
	return
}
