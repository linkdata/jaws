package jaws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/linkdata/jaws/what"
	"github.com/linkdata/jq"
)

type IsJsVar interface {
	EventHandler
	AppendJSON(b []byte, e *Element) []byte
}

type JsVar[T comparable] struct {
	locker sync.Locker
	value  *T
}

func (ui *JsVar[T]) JawsGet(elem *Element) (value T) {
	if ui.locker != nil {
		if rwl, ok := ui.locker.(RWLocker); ok {
			rwl.RLock()
			defer rwl.RUnlock()
		} else {
			ui.locker.Lock()
			defer ui.locker.Unlock()
		}
	}

	pvalue, _ := jq.GetAs[*T](ui.value, "")
	value = *pvalue
	return
}

func (ui *JsVar[T]) JawsSet(elem *Element, value T) (err error) {
	if ui.locker != nil {
		ui.locker.Lock()
		defer ui.locker.Unlock()
	}
	err = jq.Set(ui.value, "", value)
	elem.Dirty(ui)
	return
}

var (
	_ IsJsVar     = &JsVar[int]{}
	_ Setter[int] = &JsVar[int]{}
	_ UI          = &JsVar[int]{}
)

func (ui *JsVar[T]) AppendJSON(b []byte, e *Element) []byte {
	if data, err := json.Marshal(ui.JawsGet(e)); err == nil {
		bytes.ReplaceAll(data, []byte(`'`), []byte(`\u0027`))
		return append(b, data...)
	} else {
		panic(err)
	}
}

func (ui *JsVar[T]) JawsRender(e *Element, w io.Writer, params []any) (err error) {
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

func (ui *JsVar[T]) JawsUpdate(e *Element) {
	e.JsSet("", string(ui.AppendJSON(nil, e)))
}

func (ui *JsVar[T]) JawsGetTag(rq *Request) any {
	return ui.value
}

func (ui *JsVar[T]) JawsEvent(e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	if wht == what.Set {
		if jspath, jsval, found := strings.Cut(val, "\t"); found {
			var v any
			if err = json.Unmarshal([]byte(jsval), &v); err == nil {
				if ui.locker != nil {
					ui.locker.Lock()
					defer ui.locker.Unlock()
				}
				if err = jq.Set(ui.value, jspath, v); err == nil {
					e.Dirty(ui)
				}
			}
		}
	}
	return
}

func NewJsVar[T comparable](v *T, l sync.Locker) (jsvar *JsVar[T]) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		return &JsVar[T]{locker: l, value: v}
	}
	panic(fmt.Sprintf("expected non-nil pointer not %s", rv.Type().String()))
}

// JsVar binds a JsVar[T] to a named Javascript variable.
func (rq RequestWriter) JsVar(jsvarname string, jsvar any, params ...any) (err error) {
	var newparams []any
	newparams = append(newparams, jsvarname)
	newparams = append(newparams, params...)
	err = rq.UI(jsvar.(UI), newparams...)
	return
}
