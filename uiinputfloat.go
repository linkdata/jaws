package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiInput
}

func (ui *UiInputFloat) WriteHtmlInput(e *Element, w io.Writer, htmltype string) error {
	if val, ok := ui.Get(e).(float64); ok {
		return ui.UiInput.WriteHtmlInput(w, htmltype, strconv.FormatFloat(val, 'f', -1, 64), e.Jid, e.Data...)
	}
	panic("jaws: UiInputFloat: not float64")
}

func (ui *UiInputFloat) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid, val)
	}
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		err = ui.Set(e, v)
	}
	return
}
