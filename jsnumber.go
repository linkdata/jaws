package jaws

import (
	"io"
)

type JsNumber struct {
	JsVariable
	FloatSetter
}

func (ui *JsNumber) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.render(ui.FloatSetter, ui.JawsGetFloat(e), e, w, params)
}

func (ui *JsNumber) JawsUpdate(e *Element) {
	_ = e.JsSet(ui.Name, ui.JawsGetFloat(e))
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
