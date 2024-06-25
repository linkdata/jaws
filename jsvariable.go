package jaws

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
)

var ErrMissingJavascriptName = errors.New("missing Javascript name")

type JsVariable struct {
	Name string
}

func (ui *JsVariable) render(getter any, val any, e *Element, w io.Writer, params []any) (err error) {
	var buf []byte
	if buf, err = json.Marshal(val); err == nil {
		var b []byte
		b = append(b, "var "...)
		b = append(b, ui.Name...)
		b = append(b, '=')
		b = append(b, buf...)
		b = append(b, ';')
		e.ApplyGetter(getter)
		attrs := e.ApplyParams(params)
		innerHTML := template.HTML(b) //#nosec G203
		err = WriteHtmlInner(w, e.Jid(), "script", "", innerHTML, attrs...)
	}
	return
}
