package jaws

import (
	"fmt"
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiInput
}

func (ui *UiInputBool) WriteHtmlInput(e *Element, w io.Writer, htmltype string) error {
	val := ui.Get(e)
	if b, ok := val.(bool); ok {
		data := e.data
		if b {
			data = append(data, "checked")
		}
		return ui.UiInput.WriteHtmlInput(w, htmltype, "", e.Jid().String(), data...)
	}
	return fmt.Errorf("jaws: UiInputBool: expected bool, got %T", val)
}

func (ui *UiInputBool) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request(), wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		ui.Set(e, v)
	}
	return
}
