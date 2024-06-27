package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
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

func (ui *JsNumber) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		var v float64
		if v, err = strconv.ParseFloat(val, 64); err == nil {
			_, err = e.maybeDirty(ui.Tag, ui.JawsSetFloat(e, v))
		}
	}
	return
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
