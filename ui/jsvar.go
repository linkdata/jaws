package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/what"
	"github.com/linkdata/jq"
)

type PathSetter interface {
	// JawsSetPath should set the JSON object member identified by jspath to the given value.
	//
	// If the member is already the given value, it should return ErrValueUnchanged.
	JawsSetPath(elem *jaws.Element, jspath string, value any) (err error)
}

type SetPather interface {
	// JawsPathSet notifies that a JSON object member identified by jspath has been set
	// to the given value and the change has been queued for broadcast.
	JawsPathSet(elem *jaws.Element, jspath string, value any)
}

type IsJsVar interface {
	jaws.RWLocker
	jaws.UI
	jaws.EventHandler
	PathSetter
}

type JsVarMaker interface {
	JawsMakeJsVar(rq *jaws.Request) (v IsJsVar, err error)
}

var (
	_ IsJsVar          = &JsVar[int]{}
	_ jaws.Setter[int] = &JsVar[int]{}
)

type JsVar[T any] struct {
	jaws.RWLocker
	Ptr *T
	Tag any
}

func (ui *JsVar[T]) JawsGetPath(elem *jaws.Element, jspath string) (value any) {
	ui.RLock()
	defer ui.RUnlock()
	var err error
	value, err = jq.Get(ui.Ptr, jspath)
	if elem != nil {
		_ = elem.Jaws.Log(err)
	}
	return
}

func (ui *JsVar[T]) JawsGet(elem *jaws.Element) (value T) {
	anyval := ui.JawsGetPath(elem, "")
	value = *((anyval).(*T))
	return
}

func (ui *JsVar[T]) setPathLocked(elem *jaws.Element, jspath string, value any) (err error) {
	if ps, ok := ((any)(ui.Ptr).(PathSetter)); ok {
		err = ps.JawsSetPath(elem, jspath, value)
	} else {
		var changed bool
		if changed, err = jq.Set(ui.Ptr, jspath, value); err == nil && !changed {
			err = jaws.ErrValueUnchanged
		}
	}
	if err == nil && elem != nil {
		var data []byte
		if data, err = json.Marshal(value); err == nil {
			elem.Jaws.Broadcast(jaws.Message{
				Dest: ui.Tag,
				What: what.Set,
				Data: jspath + "=" + string(data),
			})
		}
	}
	return
}

func (ui *JsVar[T]) setPathLock(elem *jaws.Element, jspath string, value any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	err = ui.setPathLocked(elem, jspath, value)
	return
}

func (ui *JsVar[T]) setPath(elem *jaws.Element, jspath string, value any) (err error) {
	if err = ui.setPathLock(elem, jspath, value); err == nil {
		if sp, ok := ((any)(ui.Ptr).(SetPather)); ok {
			sp.JawsPathSet(elem, jspath, value)
		}
	}
	return
}

func (ui *JsVar[T]) JawsSetPath(elem *jaws.Element, jspath string, value any) (err error) {
	return ui.setPath(elem, jspath, value)
}

func (ui *JsVar[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	return ui.JawsSetPath(elem, "", value)
}

func appendAttrs(b []byte, attrs []template.HTMLAttr) []byte {
	for _, s := range attrs {
		if s != "" {
			b = append(b, ' ')
			b = append(b, s...)
		}
	}
	return b
}

func (ui *JsVar[T]) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	if ui.Tag, err = e.ApplyGetter(ui.Ptr); err == nil {
		var data []byte
		if ui.Ptr != nil {
			if !reflect.ValueOf(*ui.Ptr).IsZero() {
				data, err = json.Marshal(ui.Ptr)
			}
		}
		if err == nil {
			jsvarname := params[0].(string)
			attrs := e.ApplyParams(params[1:])
			var b []byte
			b = append(b, "\n<div id="...)
			b = e.Jid().AppendQuote(b)
			b = append(b, ` data-jawsname=`...)
			b = strconv.AppendQuote(b, jsvarname)
			if data != nil {
				b = append(b, ` data-jawsdata='`...)
				b = append(b, bytes.ReplaceAll(data, []byte(`'`), []byte(`\u0027`))...)
				b = append(b, "'"...)
			}
			b = appendAttrs(b, attrs)
			b = append(b, " hidden></div>"...)
			_, err = w.Write(b)
		}
	}
	return
}

func (ui *JsVar[T]) JawsGetTag(rq *jaws.Request) any {
	return ui.Tag
}

func (ui *JsVar[T]) JawsUpdate(e *jaws.Element) {} // no-op for JsVar[T]

func elideErrValueUnchanged(err error) error {
	if err == jaws.ErrValueUnchanged {
		return nil
	}
	return err
}

func (ui *JsVar[T]) JawsEvent(e *jaws.Element, wht what.What, val string) (err error) {
	err = jaws.ErrEventUnhandled
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

func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	if rl, ok := l.(jaws.RWLocker); ok {
		return &JsVar[T]{RWLocker: rl, Ptr: v}
	}
	return &JsVar[T]{RWLocker: rwlocker{l}, Ptr: v}
}

// JsVar binds a JsVar[T] to a named Javascript variable.
//
// You can also pass a JsVarMaker instead of a JsVar[T].
func (rqw RequestWriter) JsVar(jsvarname string, jsvar any, params ...any) (err error) {
	if jvm, ok := jsvar.(JsVarMaker); ok {
		jsvar, err = jvm.JawsMakeJsVar(rqw.Request)
	}
	if err == nil {
		var newparams []any
		newparams = append(newparams, jsvarname)
		newparams = append(newparams, params...)
		err = rqw.UI(jsvar.(jaws.UI), newparams...)
	}
	return
}
