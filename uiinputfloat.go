package jaws

import (
	"fmt"
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiInput
}

func (ui *UiInputFloat) WriteHtmlInput(e *Element, w io.Writer, htmltype string) error {
	val := ui.Get(e)
	if n, ok := val.(float64); ok {
		return ui.UiInput.WriteHtmlInput(w, htmltype, strconv.FormatFloat(n, 'f', -1, 64), e.Jid().String(), e.data...)
	}
	return fmt.Errorf("jaws: UiInputFloat: expected float64, got %T", val)
}

func (ui *UiInputFloat) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request(), wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		ui.Set(e, v)
	}
	return
}
