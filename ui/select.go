package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/what"
)

type Select struct {
	WrapContainer
}

func NewSelect(sh pkg.SelectHandler) *Select {
	return &Select{WrapContainer: NewWrapContainer(sh)}
}

func (ui *Select) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.RenderContainer(e, w, "select", params)
}

func (ui *Select) JawsUpdate(e *pkg.Element) {
	e.SetValue(ui.WrapContainer.Container.(pkg.Getter[string]).JawsGet(e))
	ui.UpdateContainer(e)
}

func (ui *Select) JawsEvent(e *pkg.Element, wht what.What, val string) (err error) {
	err = pkg.ErrEventUnhandled
	if wht == what.Input {
		_, err = applyDirty(ui.Tag, e, ui.WrapContainer.Container.(pkg.Setter[string]).JawsSet(e, val))
	}
	return
}
