package jaws

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/linkdata/jaws/what"
)

type IsJsFunc interface {
	UI
	IsJsFunc()
}

type JsFunc[T, U comparable] struct {
	Name string
	Arg  Binder[T]
	Retv Binder[U]
}

var _ IsJsFunc = JsFunc[int, int]{}

func (ui JsFunc[T, U]) IsJsFunc() {}

func (ui JsFunc[T, U]) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.Arg); err == nil {
		attrs := e.ApplyParams(params)
		var b []byte
		b = append(b, "\n"+`<div id=`...)
		b = e.Jid().AppendQuote(b)
		b = append(b, ` data-jawsname=`...)
		b = strconv.AppendQuote(b, ui.Name)
		b = appendAttrs(b, attrs)
		b = append(b, ` hidden></div>`+"\n"...)
		_, err = w.Write(b)
	}
	return
}

func (ui JsFunc[T, U]) JawsUpdate(e *Element) {
	v := ui.Arg.JawsGet(e)
	b, err := json.Marshal(v)
	if e.Jaws.Log(err) == nil {
		e.JsCall(string(b))
	}
}

func (ui JsFunc[T, U]) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		if _, jsval, found := strings.Cut(val, "="); found {
			var v U
			if err = json.Unmarshal([]byte(jsval), &v); err == nil {
				err = ui.Retv.JawsSet(e, v)
			}
		}
	}
	return
}

func NewJsFunc[T, U comparable](name string, arg Binder[T], retv Binder[U]) JsFunc[T, U] {
	return JsFunc[T, U]{
		Name: name,
		Arg:  arg,
		Retv: retv,
	}
}

func (rq RequestWriter) JsFunc(arg any, params ...any) (err error) {
	var jsfunc IsJsFunc
	switch arg := arg.(type) {
	case IsJsFunc:
		jsfunc = arg
	}
	err = rq.UI(jsfunc, params...)
	return
}
