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

func (ui *UiInputBool) WriteHtmlInput(e *Element, w io.Writer, htmltype string, params ...interface{}) {
	attrs := ui.parseParams(e, params)
	val := ui.Get(e)
	if b, ok := val.(bool); ok {
		if b {
			attrs = append(attrs, "checked")
		}
		writeUiDebug(e, w)
		maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, "", attrs...))
		return
	}
	panic(fmt.Errorf("jaws: UiInputBool: expected bool, got %T from %T", val, ui.ValueProxy))
}

func (ui *UiInputBool) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		ui.UiInput.Set(e, v)
	}
	return
}
