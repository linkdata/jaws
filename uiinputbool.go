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

func (ui *UiInputBool) renderBoolInput(e *Element, w io.Writer, htmltype string, params ...any) error {
	ui.applyGetter(e, ui.Setter)
	attrs := e.ApplyParams(params)
	v := ui.JawsGet(e)
	ui.Last.Store(v)
	if v {
		attrs = append(attrs, "checked")
	}
	return WriteHTMLInput(w, e.Jid(), htmltype, "", attrs)
}

func (ui *UiInputBool) JawsUpdate(e *Element) {
	v := ui.JawsGet(e)
	if ui.Last.Swap(v) != v {
		txt := "false"
		if v {
			txt = "true"
		}
		e.SetValue(txt)
	}
}

func (ui *UiInputBool) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		err = ui.maybeDirty(v, e, ui.Setter.JawsSet(e, v))
	}
	return
}
