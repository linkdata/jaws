package jaws

import (
	"io"

	"github.com/linkdata/jaws/what"
)

type JsString struct {
	JsVariable
	StringSetter
}

func (ui *JsString) JawsGetTag(rq *Request) any {
	return ui.StringSetter
}

func (ui *JsString) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.render(ui.StringSetter, ui.JawsGetString(e), e, w, params)
}

func (ui *JsString) JawsUpdate(e *Element) {
	_ = e.JsSet(ui.Name, ui.JawsGetString(e))
}

func (ui *JsString) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		_, err = e.maybeDirty(ui, ui.JawsSetString(e, val))
	}
	return
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
