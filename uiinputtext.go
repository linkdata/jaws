package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputText struct {
	UiBase
	HtmlType    string
	Value       string
	InputTextFn InputTextFn
}

func (ui *UiInputText) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return WriteHtmlInput(w, jid, ui.HtmlType, ui.Value, attrs...)
}

func (ui *UiInputText) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input && ui.InputTextFn != nil {
		err = ui.InputTextFn(rq, jid, val)
	}
	return
}
