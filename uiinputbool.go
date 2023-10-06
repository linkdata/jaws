package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiInput
	BoolGetter
}

func (ui *UiInputBool) renderBoolInput(e *Element, w io.Writer, htmltype string, params ...interface{}) {
	ui.parseGetter(e, ui.BoolGetter)
	attrs := ui.parseParams(e, params)
	v := ui.JawsGetBool(e)
	ui.Last.Store(v)
	if v {
		attrs = append(attrs, "checked")
	}
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, "", attrs...))
}

func (ui *UiInputBool) JawsUpdate(e *Element) {
	v := ui.JawsGetBool(e)
	if ui.Last.Swap(v) != v {
		if v {
			e.SetValue("true")
		} else {
			e.SetValue("false")
		}
	}
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
		ui.Last.Store(v)
		ui.BoolGetter.(BoolSetter).JawsSetBool(e, v)
		e.Dirty(ui.Tag)
	}
	return
}
