package jaws

import (
	"bytes"
	"encoding/json"
	"io"
)

type JsVariable struct {
	JsName
}

func (ui *JsVariable) render(getter any, val any, e *Element, w io.Writer, params []any) (err error) {
	var data []byte
	if val != nil {
		if data, err = json.Marshal(val); err == nil {
			data = bytes.ReplaceAll(data, []byte(`'`), []byte(`\u0027`))
		}
	}
	if err == nil {
		err = ui.JsName.render(getter, data, e, w, params)
	}
	return
}
