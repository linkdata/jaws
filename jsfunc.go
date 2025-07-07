package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type JsFunc struct {
	Arg  IsJsVar
	Retv IsJsVar
}

func (ui JsFunc) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.Arg); err == nil {
		jsvarname := params[0].(string)
		attrs := e.ApplyParams(params[1:])
		var b []byte
		b = append(b, `<div id=`...)
		b = e.Jid().AppendQuote(b)
		b = append(b, ` data-jawsname=`...)
		b = strconv.AppendQuote(b, jsvarname)
		b = appendAttrs(b, attrs)
		b = append(b, ` hidden></div>`...)
		_, err = w.Write(b)
	}
	return
}

func (ui JsFunc) JawsUpdate(e *Element) {
	e.JsCall(string(ui.Arg.AppendJSON(nil, e)))
}

func (ui JsFunc) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if ui.Retv != nil {
		err = ui.Retv.JawsEvent(e, wht, val)
	}
	return
}

func NewJsFunc(arg IsJsVar, retv IsJsVar) JsFunc {
	return JsFunc{
		Arg:  arg,
		Retv: retv,
	}
}

func (rq RequestWriter) JsFunc(jsfuncname string, getter any, params ...any) (err error) {
	var arg IsJsVar
	var retv IsJsVar
	var newparams []any

	if arg, err = makeJsVar(rq.Request(), getter); err == nil {
		newparams = append(newparams, jsfuncname)
		for _, param := range params {
			if err == nil {
				if vm, ok := param.(VarMaker); ok {
					if param, err = vm.JawsVarMake(rq.Request()); err != nil {
						return
					}
				}
				if jsvar, ok := param.(IsJsVar); ok {
					retv = jsvar
				} else {
					newparams = append(newparams, param)
				}
			}
		}
		err = rq.UI(NewJsFunc(arg, retv), newparams...)
	}
	return
}
