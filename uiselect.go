package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiSelect struct {
	uiWrapContainer
}

func NewUiSelect(sh SelectHandler) *UiSelect {
	return &UiSelect{
		uiWrapContainer{
			Container: sh,
		},
	}
}

func (ui *UiSelect) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderContainer(e, w, "select", params)
}

func (ui *UiSelect) JawsUpdate(e *Element) {
	e.SetValue(ui.uiWrapContainer.Container.(StringSetter).JawsGetString(e))
	ui.uiWrapContainer.JawsUpdate(e)
}

func (ui *UiSelect) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		_, err = e.maybeDirty(ui, ui.uiWrapContainer.Container.(StringSetter).JawsSetString(e, val))
	}
	return
}

func (rq RequestWriter) Select(sh SelectHandler, params ...any) error {
	return rq.UI(NewUiSelect(sh), params...)
}
