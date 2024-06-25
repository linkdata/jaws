package jaws

import (
	"io"
)

type JsString struct {
	JsVariable
	StringSetter
}

func (ui *JsString) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.render(ui.StringSetter, e, w, params)
}

func (ui *JsString) JawsUpdate(e *Element) {
	e.JsSet(ui.Name, ui.JawsGetString(e))
}

func NewJsString(g StringSetter, name string) *JsString {
	return &JsString{
		JsVariable:   JsVariable{Name: name},
		StringSetter: g,
	}
}

func (rq RequestWriter) JsString(value any, name string, params ...any) error {
	return rq.UI(NewJsString(makeStringSetter(value), name), params...)
}
