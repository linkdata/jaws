package jaws

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type VarMaker interface {
	// JawsVarMake must return an object that implements Setter[T] and UI, usually a JsVar[T].
	JawsVarMake(rq *Request) (UI, error)
}

type JsVar[T comparable] struct {
	Setter[T] // typed generic Setter
}

var (
	_ Setter[int] = JsVar[int]{}
	_ UI          = JsVar[int]{}
)

func (ui JsVar[T]) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	e.ApplyGetter(ui.Setter)
	jsvarname := params[len(params)-1].(string)
	attrs := e.ApplyParams(params[:len(params)-1])
	var b []byte
	b = append(b, `<div id=`...)
	b = e.Jid().AppendQuote(b)

	var data []byte
	if data, err = json.Marshal(ui.JawsGet(e)); err == nil {
		data = bytes.ReplaceAll(data, []byte(`'`), []byte(`\u0027`))
		b = append(b, ` data-jawsdata='`...)
		b = append(b, data...)
		b = append(b, '\'')
	}
	b = append(b, ` data-jawsname=`...)
	b = strconv.AppendQuote(b, jsvarname)
	b = appendAttrs(b, attrs)
	b = append(b, ` hidden></div>`...)
	_, err = w.Write(b)
	return
}

func (ui JsVar[T]) JawsUpdate(e *Element) {
	_ = e.JsSet(ui.JawsGet(e))
}

func (ui JsVar[T]) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		var v T
		if err = json.Unmarshal([]byte(val), &v); err == nil {
			_, err = e.maybeDirty(ui.Setter, ui.JawsSet(e, v))
		}
	}
	return
}

func NewJsVar[T comparable](setter Setter[T]) (v JsVar[T]) {
	return JsVar[T]{Setter: setter}
}

// JsVar binds a Setter[T] to a named Javascript variable.
//
// Alternatively you may also pass a VarMaker that returns an object that
// implements Setter[T] and UI.
func (rq RequestWriter) JsVar(setter any, jsvarname string, params ...any) (err error) {
	if vm, ok := setter.(VarMaker); ok {
		setter, err = vm.JawsVarMake(rq.Request())
	}
	if err == nil {
		params = append(params, jsvarname)
		err = rq.UI(setter.(UI), params...)
	}
	return
}
