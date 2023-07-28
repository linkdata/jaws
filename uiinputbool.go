package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiInput
}

func (ui *UiInputBool) WriteHtmlInput(e *Element, w io.Writer, htmltype string) error {
	if val, ok := ui.Get(e).(bool); ok {
		data := e.Data
		if val {
			data = append(data, "checked")
		}
		return ui.UiInput.WriteHtmlInput(w, htmltype, "", e.Jid(), data...)
	}
	panic("jaws: UiInputBool: not bool")
}

func (ui *UiInputBool) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request(), wht, e.Jid(), val)
	}
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		err = ui.Set(e, v)
	}
	return
}
