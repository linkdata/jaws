package jaws

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
)

type JsVariable struct {
	Tag  any
	Name string
}

func (ui *JsVariable) JawsGetTag(rq *Request) any {
	return ui.Tag
}

func (ui *JsVariable) render(getter any, val any, e *Element, w io.Writer, params []any) (err error) {
	var data []byte
	if data, err = json.Marshal(val); err == nil {
		data = bytes.ReplaceAll(data, []byte(`'`), []byte(`\u0027`))
		ui.Tag = e.ApplyGetter(getter)
		attrs := e.ApplyParams(params)
		var b []byte
		b = append(b, `<div id=`...)
		b = e.Jid().AppendQuote(b)
		b = append(b, ` data-jawsdata='`...)
		b = append(b, data...)
		b = append(b, `' data-jawsname=`...)
		b = strconv.AppendQuote(b, ui.Name)
		b = appendAttrs(b, attrs)
		b = append(b, ` hidden></div>`...)
		_, err = w.Write(b)
	}
	return
}
