package jaws

import (
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type JsFunc struct {
	Arg  isJsVar
	Retv EventHandler
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

func NewJsFunc(arg isJsVar, retv EventHandler) JsFunc {
	return JsFunc{
		Arg:  arg,
		Retv: retv,
	}
}

func (rq RequestWriter) JsFunc(jsfuncname string, arg any, params ...any) (err error) {
	var retv EventHandler
	var newparams []any
	newparams = append(newparams, jsfuncname)
	for _, param := range params {
		if jsvar, ok := param.(isJsVar); ok {
			retv = jsvar
		} else {
			newparams = append(newparams, param)
		}
	}
	err = rq.UI(NewJsFunc(arg.(isJsVar), retv), newparams...)
	return
}
