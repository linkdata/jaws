package jaws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type VarMaker interface {
	// JawsVarMake must return an object that implements IsJsVar, Setter[T] and UI, usually a JsVar[T].
	JawsVarMake(rq *Request) (IsJsVar, error)
}

type IsJsVar interface {
	JawsIsJsVar()
	EventHandler
	AppendJSON(b []byte, e *Element) []byte
}

type JsVar[T comparable] struct {
	Setter[T] // typed generic Setter
}

var (
	_ IsJsVar     = JsVar[int]{}
	_ Setter[int] = JsVar[int]{}
	_ UI          = JsVar[int]{}
)

func (ui JsVar[T]) JawsIsJsVar() {
}

func (ui JsVar[T]) AppendJSON(b []byte, e *Element) []byte {
	if data, err := json.Marshal(ui.JawsGet(e)); err == nil {
		bytes.ReplaceAll(data, []byte(`'`), []byte(`\u0027`))
		return append(b, data...)
	} else {
		panic(err)
	}
}

func (ui JsVar[T]) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	e.ApplyGetter(ui.Setter)
	jsvarname := params[0].(string)
	attrs := e.ApplyParams(params[1:])
	var b []byte
	b = append(b, `<div id=`...)
	b = e.Jid().AppendQuote(b)
	b = append(b, ` data-jawsdata='`...)
	b = ui.AppendJSON(b, e)
	b = append(b, `' data-jawsname=`...)
	b = strconv.AppendQuote(b, jsvarname)
	b = appendAttrs(b, attrs)
	b = append(b, ` hidden></div>`...)
	_, err = w.Write(b)
	return
}

func (ui JsVar[T]) JawsUpdate(e *Element) {
	e.JsSet(string(ui.AppendJSON(nil, e)))
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

func makeJsVar(rq *Request, v any) (IsJsVar, error) {
	switch v := v.(type) {
	case VarMaker:
		return v.JawsVarMake(rq)
	case IsJsVar:
		return v, nil
	}
	panic(fmt.Sprintf("expected IsJsVar or VarMaker, not %T", v))
}

// JsVar binds a Setter[T] to a named Javascript variable.
//
// Alternatively you may also pass a VarMaker that returns an object that
// implements Setter[T] and UI.
func (rq RequestWriter) JsVar(jsvarname string, setter any, params ...any) (err error) {
	var newparams []any
	if setter, err = makeJsVar(rq.Request(), setter); err == nil {
		newparams = append(newparams, jsvarname)
		newparams = append(newparams, params...)
		err = rq.UI(setter.(UI), newparams...)
	}
	return
}
