package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiHtml
	HtmlType     string
	Value        float64
	InputFloatFn InputFloatFn
}

func (ui *UiInputFloat) WriteHtmlInput(rq *Request, w io.Writer, htmltype, jid string, data ...interface{}) error {
	return ui.UiHtml.WriteHtmlInput(rq, w, htmltype, strconv.FormatFloat(ui.Value, 'f', -1, 64), jid, data...)
}

func (ui *UiInputFloat) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		if ui.InputFloatFn != nil {
			if err = ui.InputFloatFn(rq, jid, v); err != nil {
				return
			}
		}
		ui.Value = v
	}
	return
}
