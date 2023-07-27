package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type UiInputSelect struct {
	UiHtml
	*NamedBoolArray
	InputTextFn InputTextFn
}

func (ui *UiInputSelect) JawsRender(rq *Request, w io.Writer, jid string, data ...interface{}) error {
	var attrs []string
	for _, v := range data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return WriteHtmlSelect(w, jid, ui.NamedBoolArray, attrs...)
}

func (ui *UiInputSelect) JawsEvent(rq *Request, wht what.What, jid, val string) (err error) {
	if wht == what.Input && ui.InputTextFn != nil {
		err = ui.InputTextFn(rq, jid, val)
	}
	return
}
