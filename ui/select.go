package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
)

type Select struct {
	WrapContainer
}

func NewSelect(sh core.SelectHandler) *Select {
	return &Select{WrapContainer: NewWrapContainer(sh)}
}

func (ui *Select) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "select", params)
}

func (ui *Select) JawsUpdate(e *core.Element) {
	e.SetValue(ui.WrapContainer.Container.(core.Getter[string]).JawsGet(e))
	ui.UpdateContainer(e)
}

func (ui *Select) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		_, err = applyDirty(ui.Tag, e, ui.WrapContainer.Container.(core.Setter[string]).JawsSet(e, val))
	}
	return
}
