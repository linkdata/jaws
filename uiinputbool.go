package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiInput
	Setter[bool]
}

func (ui *UiInputBool) renderBoolInput(e ElementIf, w io.Writer, htmltype string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		if v {
			attrs = append(attrs, "checked")
		}
		err = WriteHTMLInput(w, e.Jid(), htmltype, "", attrs)
	}
	return
}

func (ui *UiInputBool) JawsUpdate(e ElementIf) {
	v := ui.JawsGet(e)
	if ui.Last.Swap(v) != v {
		txt := "false"
		if v {
			txt = "true"
		}
		e.SetValue(txt)
	}
}

func (ui *UiInputBool) JawsEvent(e ElementIf, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		err = ui.MaybeDirty(v, e, ui.Setter.JawsSet(e, v))
	}
	return
}
