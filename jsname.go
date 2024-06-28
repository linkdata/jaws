package jaws

import (
	"io"
	"strconv"
)

type JsName struct {
	Tag  any
	Name string
}

func (ui *JsName) JawsGetTag(rq *Request) any {
	return ui.Tag
}

func (ui *JsName) render(getter any, data []byte, e *Element, w io.Writer, params []any) (err error) {
	ui.Tag = e.ApplyGetter(getter)
	attrs := e.ApplyParams(params)
	var b []byte
	b = append(b, `<div id=`...)
	b = e.Jid().AppendQuote(b)
	if len(data) > 0 {
		b = append(b, ` data-jawsdata='`...)
		b = append(b, data...)
		b = append(b, '\'')
	}
	b = append(b, ` data-jawsname=`...)
	b = strconv.AppendQuote(b, ui.Name)
	b = appendAttrs(b, attrs)
	b = append(b, ` hidden></div>`...)
	_, err = w.Write(b)
	return
}
