package jaws

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var ErrMissingJavascriptName = errors.New("missing Javascript name")

type JsVariable struct {
	Tag  any
	Name string
}

func (ui *JsVariable) JawsGetTag(rq *Request) any {
	return ui.Tag
}

func (ui *JsVariable) render(getter any, val any, e *Element, w io.Writer, params []any) (err error) {
	var buf []byte
	if buf, err = json.Marshal(val); err == nil {
		buf = bytes.ReplaceAll(buf, []byte(`'`), []byte(`\u0027`))
		ui.Tag = e.ApplyGetter(getter)
		attrs := e.ApplyParams(params)
		var b []byte
		b = append(b, `<div id="Jvar.`...)
		b = append(b, ui.Name...)
		b = append(b, `" data-json='`...)
		b = append(b, buf...)
		b = append(b, `' data-jid=`...)
		b = e.Jid().AppendQuote(b)
		b = appendAttrs(b, attrs)
		b = append(b, `></div>`...)
		_, err = w.Write(b)
	}
	return
}
