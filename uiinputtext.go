package jaws

import (
	"fmt"
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiInput
}

func (ui *UiInputText) WriteHtmlInput(e *Element, w io.Writer, htmltype string) error {
	val := ui.Get(e)
	if s, ok := val.(string); ok {
		writeUiDebug(e, w)
		return ui.UiInput.WriteHtmlInput(w, e, htmltype, s, e.Data...)
	}
	return fmt.Errorf("jaws: UiInputText: expected string, got %T", val)
}

func (ui *UiInputText) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		ui.Set(e, val)
	}
	return
}
