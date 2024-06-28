package jaws

import (
	"encoding/json"
	"io"

	"github.com/linkdata/jaws/what"
)

type JsAny struct {
	JsVariable
	AnySetter
}

func (ui *JsAny) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.render(ui.AnySetter, ui.JawsGetAny(e), e, w, params)
}

func (ui *JsAny) JawsUpdate(e *Element) {
	_ = e.JsSet(ui.JawsGetAny(e))
}

func (ui *JsAny) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		var v any
		if err = json.Unmarshal([]byte(val), &v); err == nil {
			_, err = e.maybeDirty(ui.Tag, ui.JawsSetAny(e, v))
		}
	}
	return
}

func NewJsAny(g AnySetter, name string) *JsAny {
	return &JsAny{
		JsVariable: JsVariable{JsName{Name: name}},
		AnySetter:  g,
	}
}

func (rq RequestWriter) JsAny(value any, name string, params ...any) error {
	return rq.UI(NewJsAny(makeAnySetter(value), name), params...)
}
