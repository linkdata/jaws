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

func (ui *UiInputBool) JawsEvent(e *Element, wht what.What, val string) (stop bool, err error) {
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		ui.Last.Store(v)
		err = ui.BoolGetter.(BoolSetter).JawsSetBool(e, v)
		e.Dirty(ui.Tag)
		if err != nil {
			return
		}
	}
	return ui.UiHtml.JawsEvent(e, wht, val)
}
