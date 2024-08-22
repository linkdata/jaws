package jaws

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

	"github.com/linkdata/jaws/what"
)

type VarNamer interface {
	JawsVarName() string
}

type JsVar[T comparable] struct {
	Name string // Javascript variable name path
	Setter[T]
}

func (ui JsVar[T]) JawsVarName() string {
	return ui.Name
}

func (ui JsVar[T]) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	e.ApplyGetter(ui.Setter)
	attrs := e.ApplyParams(params)
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
	b = strconv.AppendQuote(b, ui.Name)
	b = appendAttrs(b, attrs)
	b = append(b, ` hidden></div>`...)
	_, err = w.Write(b)
	return
}

func (ui JsVar[T]) JawsUpdate(e *Element) {
	_ = e.JsSet(ui.JawsGet(e))
}

func (ui JsVar[T]) JawsGetTag(rq *Request) any {
	return ui.Setter
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

func NewJsVar[T comparable](name string, setter Setter[T]) (v JsVar[T]) {
	return JsVar[T]{
		Name:   name,
		Setter: setter,
	}
}

func (rq RequestWriter) JsVar(jsVar VarNamer) error {
	return rq.UI(jsVar.(UI), nil)
}
