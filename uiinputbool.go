package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputBool struct {
	UiHtml
	BoolGetter
}

func (ui *UiInputBool) renderBoolInput(e *Element, w io.Writer, htmltype string, params ...interface{}) {
	ui.parseGetter(e, ui.BoolGetter)
	attrs := ui.parseParams(e, params)
	b := ui.JawsGetBool(e)
	if b {
		attrs = append(attrs, "checked")
	}
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, "", attrs...))
}

func (ui *UiInputBool) JawsUpdate(e *Element) {
	if ui.JawsGetBool(e) {
		e.SetAttr("checked", "")
	} else {
		e.RemoveAttr("checked")
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
		err = ui.BoolGetter.(BoolSetter).JawsSetBool(e, v)
		e.Dirty(ui.Tag)
	}
	return
}
