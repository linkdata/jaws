package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiHtml
	FloatGetter
}

func (ui *UiInputFloat) value(e *Element) string {
	return strconv.FormatFloat(ui.JawsGetFloat(e), 'f', -1, 64)
}

func (ui *UiInputFloat) renderFloatInput(e *Element, w io.Writer, htmltype string, params ...interface{}) {
	ui.parseGetter(e, ui.FloatGetter)
	attrs := ui.parseParams(e, params)
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInput(w, e.Jid(), htmltype, ui.value(e), attrs...))
}

func (ui *UiInputFloat) JawsUpdate(u Updater) {
	u.SetValue(ui.value(u.Element))
}

func (ui *UiInputFloat) JawsEvent(e *Element, wht what.What, val string) (err error) {
	if ui.EventFn != nil {
		return ui.EventFn(e.Request, wht, e.Jid().String(), val)
	}
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		err = ui.FloatGetter.(FloatSetter).JawsSetFloat(e, v)
		e.Jaws.Dirty(ui.Tag)
	}
	return
}
