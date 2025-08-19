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

type PathSetter interface {
	// JawsSetPath should set the JSON object member identified by jspath to the given value.
	//
	// If the member is already the given value, it should return ErrValueUnchanged.
	JawsSetPath(elem *Element, jspath string, value any) (err error)
}

type SetPather interface {
	// JawsPathSet notifies that a JSON object member identified by jspath has been set
	// to the given value and the change has been queued for broadcast.
	JawsPathSet(elem *Element, jspath string, value any)
}

type IsJsVar interface {
	RWLocker
	UI
	EventHandler
	PathSetter
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
	tag any
}

func (ui *JsVar[T]) JawsGetPath(elem *Element, jspath string) (value any) {
	ui.RLock()
	defer ui.RUnlock()
	var err error
	value, err = jq.Get(ui.ptr, jspath)
	if elem != nil {
		_ = elem.Jaws.Log(err)
	}
	return
}

func (ui *JsVar[T]) JawsGet(elem *Element) (value T) {
	anyval := ui.JawsGetPath(elem, "")
	value = *((anyval).(*T))
	return
}

func (ui *JsVar[T]) setPathLocked(elem *Element, jspath string, value any) (err error) {
	if ps, ok := ((any)(ui.ptr).(PathSetter)); ok {
		err = ps.JawsSetPath(elem, jspath, value)
	} else {
		var changed bool
		if changed, err = jq.Set(ui.ptr, jspath, value); err == nil && !changed {
			err = ErrValueUnchanged
		}
	}
	if err == nil && elem != nil {
		var data []byte
		if data, err = json.Marshal(value); err == nil {
			elem.Jaws.Broadcast(Message{
				Dest: ui.tag,
				What: what.Set,
				Data: jspath + "=" + string(data),
			})
		}
	}
	return
}

func (ui *JsVar[T]) setPathLock(elem *Element, jspath string, value any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	err = ui.setPathLocked(elem, jspath, value)
	return
}

func (ui *JsVar[T]) setPath(elem *Element, jspath string, value any) (err error) {
	if err = ui.setPathLock(elem, jspath, value); err == nil {
		if sp, ok := ((any)(ui.ptr).(SetPather)); ok {
			sp.JawsPathSet(elem, jspath, value)
		}
	}
	return
}

func (ui *JsVar[T]) JawsSetPath(elem *Element, jspath string, value any) (err error) {
	return ui.setPath(elem, jspath, value)
}

func (ui *JsVar[T]) JawsSet(elem *Element, value T) (err error) {
	return ui.JawsSetPath(elem, "", value)
}

func (ui *JsVar[T]) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	if ui.tag, err = e.ApplyGetter(ui.ptr); err == nil {
		var data []byte
		if data, err = json.Marshal(ui.ptr); err == nil {
			jsvarname := params[0].(string)
			attrs := e.ApplyParams(params[1:])
			var b []byte
			b = append(b, `<div id=`...)
			b = e.Jid().AppendQuote(b)
			b = append(b, ` data-jawsname=`...)
			b = strconv.AppendQuote(b, jsvarname)
			b = append(b, ` data-jawsdata='`...)
			b = append(b, bytes.ReplaceAll(data, []byte(`'`), []byte(`\u0027`))...)
			b = append(b, "'"...)
			b = appendAttrs(b, attrs)
			b = append(b, ` hidden></div>`...)
			_, err = w.Write(b)
		}
	}
	return
}

func (ui *JsVar[T]) JawsGetTag(rq *Request) any {
	return ui.tag
}

func (ui *JsVar[T]) JawsUpdate(e *Element) {} // no-op for JsVar[T]

func elideErrValueUnchanged(err error) error {
	if err == ErrValueUnchanged {
		return nil
	}
	return err
}

func (ui *JsVar[T]) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		if jspath, jsval, found := strings.Cut(val, "="); found {
			var v any
			if err = json.Unmarshal([]byte(jsval), &v); err == nil {
				err = elideErrValueUnchanged(ui.setPath(e, jspath, v))
			}
		}
	}
	return
}

// NewJsVar creates a binding with a Locker (or RWLocker) and
// pointer to underlying data.
//
// JsVar's use JawsRender, and that rendering will contain the
// JSON representation of the underlying data. This will be used to
// initialize the named Javascript variable before "DOMContentLoaded"
// fires. Note that we don't render the Javascript variable declaration,
// you'll have to do that yourself.
//
// JsVar's do *NOT* use JawsUpdate, so changing the underlying data and
// calling JawsUpdate will have no effect. Instead, JsVar's are
// synchronized across browsers using immediate broadcasts.
//
// Changes to JsVar's should be made using their [JawsSet] or
// [JawsSetPath] methods. If *T implements [PathSetter],
// that will be used instead of jq.Set().
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
