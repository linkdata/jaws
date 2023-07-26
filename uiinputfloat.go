package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type UiInputFloat struct {
	UiBase
	HtmlType     string
	Value        float64
	InputFloatFn InputFloatFn
}

func (ui *UiInputFloat) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return WriteHtmlInput(w, jid, ui.HtmlType, strconv.FormatFloat(ui.Value, 'f', -1, 64), attrs...)
}

func (ui *UiInputFloat) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input && ui.InputFloatFn != nil {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		err = ui.InputFloatFn(rq, jid, v)
	}
	return
}
