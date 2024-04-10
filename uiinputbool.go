package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiInput
	BoolSetter
}

func (ui *UiInputBool) renderBoolInput(e *Element, w io.Writer, htmltype string, params ...any) error {
	ui.parseGetter(e, ui.BoolSetter)
	attrs := e.ApplyParams(params)
	v := ui.JawsGetBool(e)
	ui.Last.Store(v)
	if v {
		attrs = append(attrs, "checked")
	}
	return WriteHtmlInput(w, e.Jid(), htmltype, "", attrs)
}

func (ui *UiInputBool) JawsUpdate(e *Element) {
	v := ui.JawsGetBool(e)
	if ui.Last.Swap(v) != v {
		txt := "false"
		if v {
			txt = "true"
		}
		e.SetValue(txt)
	}
}

func (ui *UiInputBool) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if wht == what.Input {
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		ui.Last.Store(v)
		err = ui.BoolSetter.JawsSetBool(e, v)
		e.Dirty(ui.Tag)
		if err != nil {
			return
		}
	}
	return ui.UiHtml.JawsEvent(e, wht, val)
}
