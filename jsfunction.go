package jaws

import (
	"encoding/json"
	"io"

	"github.com/linkdata/jaws/what"
)

type JsFunction struct {
	JsName
	Param     AnySetter
	Result    AnySetter
	ResultTag any
}

func (ui *JsFunction) JawsRender(e *Element, w io.Writer, params []any) error {
	ui.ResultTag = e.ApplyGetter(ui.Result)
	return ui.render(ui.Param, nil, e, w, params)
}

func (ui *JsFunction) JawsUpdate(e *Element) {
	_ = e.JsCall(ui.Param.JawsGetAny(e))
}

func (ui *JsFunction) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		var v any
		if err = json.Unmarshal([]byte(val), &v); err == nil {
			_, err = e.maybeDirty(ui.ResultTag, ui.Result.JawsSetAny(e, v))
		}
	}
	return
}

func NewJsFunction(param, result AnySetter, name string) *JsFunction {
	if param == nil {
		panic("NewJsFunction param is nil")
	}
	return &JsFunction{
		JsName: JsName{Name: name},
		Param:  param,
		Result: result,
	}
}

func (rq RequestWriter) JsFunction(param, result any, name string, params ...any) error {
	return rq.UI(NewJsFunction(makeAnySetter(param), makeAnySetter(result), name), params...)
}
