package jaws

import (
	"io"
	"strconv"
)

type JsNumber struct {
	JsVariable
	FloatSetter
}

func (ui *JsNumber) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.render(ui.FloatSetter, e, w, params)
}

func (ui *JsNumber) JawsUpdate(e *Element) {
	e.JsSet(ui.Name, strconv.FormatFloat(ui.JawsGetFloat(e), 'f', -1, 64))
}

func NewJsNumber(g FloatSetter, name string) *JsNumber {
	return &JsNumber{
		JsVariable:  JsVariable{Name: name},
		FloatSetter: g,
	}
}

func (rq RequestWriter) JsNumber(value any, name string, params ...any) error {
	return rq.UI(NewJsNumber(makeFloatSetter(value), name), params...)
}
