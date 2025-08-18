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

/*
	JsVar's use JawsRender, and that rendering will contain the
	JSON representation of the underlying data. This will be used to
	initialize the named Javascript variable before "DOMContentLoaded"
	fires. Note that we don't render the Javascript variable declaration,
	you'll have to do that yourself.

	JsVar's do *NOT* use JawsUpdate, so changing the underlying data and
	calling JawsUpdate will have no effect. Instead, JsVar's are
	synchronized across browsers using immediate broadcasts.

	Changes to JsVar's should be made using their [JawsSet] or
	[JawsSetPath] methods.
*/

type IsJsVar interface {
	RWLocker
	UI
	EventHandler
	AppendJSONLocked(b []byte, e *Element) []byte
}

type JsVarMaker interface {
	JawsMakeJsVar(rq *Request) (v IsJsVar, err error)
}

var (
	_ IsJsVar     = &JsVar[int]{}
	_ Setter[int] = &JsVar[int]{}
)

type JsVar[T any] struct {
	RWLocker
	ptr *T
}

func (ui *JsVar[T]) JawsGetPath(elem *Element, jspath string) (value any) {
	ui.RLock()
	defer ui.RUnlock()
	var err error
	value, err = jq.Get(ui.ptr, jspath)
	_ = elem.Jaws.Log(err)
	return
}

func (ui *JsVar[T]) JawsGet(elem *Element) (value T) {
	anyval := ui.JawsGetPath(elem, "")
	value = *((anyval).(*T))
	return
}

func (ui *JsVar[T]) JawsSetPath(elem *Element, jspath string, value any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	var changed bool
	if changed, err = jq.Set(ui.ptr, jspath, value); changed {
		var data []byte
		if data, err = json.Marshal(value); err == nil {
			elem.Jaws.Broadcast(Message{
				Dest: ui.ptr,
				What: what.Set,
				Data: jspath + "=" + string(data),
			})
		}
	}
	return
}

func (ui *JsVar[T]) JawsSet(elem *Element, value T) (err error) {
	return ui.JawsSetPath(elem, "", value)
}

func (ui *JsVar[T]) AppendJSONLocked(b []byte, e *Element) []byte {
	if data, err := json.Marshal(ui.ptr); err == nil {
		return append(b, data...)
	} else {
		panic(err)
	}
}

func (ui *JsVar[T]) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	if _, err = e.ApplyGetter(ui.ptr); err == nil {
		jsvarname := params[0].(string)
		attrs := e.ApplyParams(params[1:])
		var b []byte
		b = append(b, `<div id=`...)
		b = e.Jid().AppendQuote(b)
		b = append(b, ` data-jawsname=`...)
		b = strconv.AppendQuote(b, jsvarname)
		b = append(b, ` data-jawsdata='`...)
		b = append(b, bytes.ReplaceAll(ui.AppendJSONLocked(nil, e), []byte(`'`), []byte(`\u0027`))...)
		b = append(b, "'"...)
		b = appendAttrs(b, attrs)
		b = append(b, ` hidden></div>`...)
		_, err = w.Write(b)
	}
	return
}

func (ui *JsVar[T]) JawsUpdate(e *Element) {} // no-op for JsVar[T]

func (ui *JsVar[T]) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		if jspath, jsval, found := strings.Cut(val, "="); found {
			var v any
			if err = json.Unmarshal([]byte(jsval), &v); err == nil {
				ui.Lock()
				defer ui.Unlock()
				var changed bool
				if changed, err = jq.Set(ui.ptr, jspath, v); changed && err == nil {
					e.Jaws.Broadcast(Message{
						Dest: []any{ui.ptr, ExceptRequest(e.Request)},
						What: what.Set,
						Data: val,
					})
				}
			}
		}
	}
	return
}

func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	if rl, ok := l.(RWLocker); ok {
		return &JsVar[T]{RWLocker: rl, ptr: v}
	}
	return &JsVar[T]{RWLocker: rwlocker{l}, ptr: v}
}

// JsVar binds a JsVar[T] to a named Javascript variable.
//
// You can also pass a JsVarMaker instead of a JsVar[T].
func (rq RequestWriter) JsVar(jsvarname string, jsvar any, params ...any) (err error) {
	if jvm, ok := jsvar.(JsVarMaker); ok {
		jsvar, err = jvm.JawsMakeJsVar(rq.Request())
	}
	if err == nil {
		var newparams []any
		newparams = append(newparams, jsvarname)
		newparams = append(newparams, params...)
		err = rq.UI(jsvar.(UI), newparams...)
	}
	return
}
