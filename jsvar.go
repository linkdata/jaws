package jaws

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/linkdata/jaws/what"
	"github.com/linkdata/jq"
)

type isJsVar interface {
	UI
	EventHandler
	AppendJSON(b []byte, e *Element) []byte
}

var (
	_ isJsVar     = JsVar[int]{}
	_ Setter[int] = JsVar[int]{}
)

type JsVar[T any] struct {
	RWLocker
	ptr *T
}

func (ui JsVar[T]) JawsGet(elem *Element) (value T) {
	ui.RLock()
	defer ui.RUnlock()
	pvalue, _ := jq.GetAs[*T](ui.ptr, "")
	value = *pvalue
	return
}

func (ui JsVar[T]) JawsSet(elem *Element, value T) (err error) {
	ui.Lock()
	defer ui.Unlock()
	err = jq.Set(ui.ptr, "", value)
	return
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
	if _, err = e.ApplyGetter(ui); err == nil {
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
	}
	return
}

func (ui JsVar[T]) JawsUpdate(e *Element) {
	e.JsSet("", string(ui.AppendJSON(nil, e)))
}

func (ui JsVar[T]) JawsGetTag(rq *Request) any {
	return ui.ptr
}

func (ui JsVar[T]) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		if jspath, jsval, found := strings.Cut(val, "\t"); found {
			var v any
			if err = json.Unmarshal([]byte(jsval), &v); err == nil {
				ui.Lock()
				defer ui.Unlock()
				if err = jq.Set(ui.ptr, jspath, v); err == nil {
					e.Dirty(ui)
				}
			}
		}
	}
	return
}

func NewJsVar[T any](l sync.Locker, v *T) JsVar[T] {
	if rl, ok := l.(RWLocker); ok {
		return JsVar[T]{RWLocker: rl, ptr: v}
	}
	return JsVar[T]{RWLocker: rwlocker{l}, ptr: v}
}

// JsVar binds a JsVar[T] to a named Javascript variable.
func (rq RequestWriter) JsVar(jsvarname string, jsvar any, params ...any) (err error) {
	var newparams []any
	newparams = append(newparams, jsvarname)
	newparams = append(newparams, params...)
	err = rq.UI(jsvar.(UI), newparams...)
	return
}
