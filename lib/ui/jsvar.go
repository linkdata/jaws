package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/jq"
)

var jsVarNameRx = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func validateJsVarName(v []any) (name string, err error) {
	if len(v) > 0 {
		var ok bool
		if name, ok = v[0].(string); !ok {
			err = errIllegalJsVarName("expected string")
			return
		}
		if !jsVarNameRx.MatchString(name) {
			err = errIllegalJsVarName("illegal syntax")
		}
	}
	if name == "" {
		err = errIllegalJsVarName("missing")
	}
	return
}

// PathSetter can set a nested JSON path value.
type PathSetter interface {
	// JawsSetPath should set the JSON object member identified by jspath to the given value.
	//
	// If the member is already the given value, it should return [jaws.ErrValueUnchanged].
	JawsSetPath(elem *jaws.Element, jspath string, value any) (err error)
}

// SetPather is notified after a nested JSON path value has been set and
// broadcast.
type SetPather interface {
	// JawsPathSet notifies that a JSON object member identified by jspath has been set
	// to the given value and the change has been queued for broadcast.
	JawsPathSet(elem *jaws.Element, jspath string, value any)
}

// IsJsVar is implemented by JaWS UI values that bind a Go value to a
// browser-side JavaScript variable.
type IsJsVar interface {
	bind.RWLocker
	jaws.UI
	jaws.InputHandler
	PathSetter
}

// JsVarMaker creates a request-scoped JavaScript variable binding.
type JsVarMaker interface {
	JawsMakeJsVar(rq *jaws.Request) (v IsJsVar, err error)
}

var (
	_ IsJsVar          = &JsVar[int]{}
	_ bind.Setter[int] = &JsVar[int]{}
)

// JsVar binds a Go value to a named JavaScript variable in the browser.
//
// It is safe for concurrent use when the locker passed to [NewJsVar] is safe
// for concurrent use.
type JsVar[T any] struct {
	bind.RWLocker
	Ptr *T  // bound Go value
	Tag any // current dirty tag
}

// JawsGetPath returns the value at jspath, logging lookup errors on elem when possible.
func (u *JsVar[T]) JawsGetPath(elem *jaws.Element, jspath string) (value any) {
	u.RLock()
	defer u.RUnlock()
	var err error
	value, err = jq.Get(u.Ptr, jspath)
	if elem != nil {
		_ = elem.Jaws.Log(err)
	}
	return
}

// JawsGet returns the bound value.
func (u *JsVar[T]) JawsGet(elem *jaws.Element) (value T) {
	u.RLock()
	defer u.RUnlock()
	if u.Ptr != nil {
		value = *u.Ptr
	}
	return
}

func (u *JsVar[T]) setPathLocked(elem *jaws.Element, jspath string, value any) (err error) {
	if ps, ok := ((any)(u.Ptr).(PathSetter)); ok {
		err = ps.JawsSetPath(elem, jspath, value)
	} else {
		var changed bool
		if changed, err = jq.Set(u.Ptr, jspath, value); err == nil && !changed {
			err = jaws.ErrValueUnchanged
		}
	}
	if err == nil && elem != nil {
		var data []byte
		if data, err = json.Marshal(value); err == nil {
			elem.Jaws.Broadcast(wire.Message{
				Dest: u.Tag,
				What: what.Set,
				Data: jspath + "=" + string(data),
			})
		}
	}
	return
}

func (u *JsVar[T]) setPathLock(elem *jaws.Element, jspath string, value any) (err error) {
	u.Lock()
	defer u.Unlock()
	err = u.setPathLocked(elem, jspath, value)
	return
}

func (u *JsVar[T]) setPath(elem *jaws.Element, jspath string, value any) (err error) {
	if err = u.setPathLock(elem, jspath, value); err == nil {
		if sp, ok := ((any)(u.Ptr).(SetPather)); ok {
			sp.JawsPathSet(elem, jspath, value)
		}
	}
	return
}

// JawsSetPath sets the value at jspath and broadcasts the change.
func (u *JsVar[T]) JawsSetPath(elem *jaws.Element, jspath string, value any) (err error) {
	return u.setPath(elem, jspath, value)
}

// JawsSet replaces the root value and broadcasts the change.
func (u *JsVar[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	return u.JawsSetPath(elem, "", value)
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

// JawsRender writes the hidden element that seeds and routes the JavaScript variable.
func (u *JsVar[T]) JawsRender(e *jaws.Element, w io.Writer, params []any) (err error) {
	u.Lock()
	defer u.Unlock()
	var getterAttrs []template.HTMLAttr
	if u.Tag, getterAttrs, err = e.ApplyGetter(u.Ptr); err == nil {
		e.AddHandlers(u)
		var jsvarname string
		if jsvarname, err = validateJsVarName(params); err == nil {
			var data []byte
			if u.Ptr != nil {
				data, err = json.Marshal(u.Ptr)
			}
			if err == nil {
				attrs := append(e.ApplyParams(params[1:]), getterAttrs...)
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
	}
	return
}

// JawsGetTag returns the current dirty tag.
func (u *JsVar[T]) JawsGetTag(tag.Context) any {
	return u.Tag
}

// JawsUpdate is a no-op because updates are broadcast by path setters.
func (u *JsVar[T]) JawsUpdate(e *jaws.Element) {
	_ = e // no-op for JsVar[T]
}

func elideErrValueUnchanged(err error) error {
	if err == jaws.ErrValueUnchanged {
		return nil
	}
	return err
}

// JawsInput applies a browser-side JavaScript variable update.
func (u *JsVar[T]) JawsInput(e *jaws.Element, val string) (err error) {
	err = jaws.ErrEventUnhandled
	if jspath, jsval, found := strings.Cut(val, "="); found {
		var v any
		if err = json.Unmarshal([]byte(jsval), &v); err == nil {
			err = elideErrValueUnchanged(u.setPath(e, jspath, v))
		}
	}
	return
}

// NewJsVar creates a JsVar over v protected by l.
//
// The locker l must be non-nil and must remain valid for the lifetime of the JsVar.
func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	if rl, ok := l.(bind.RWLocker); ok {
		return &JsVar[T]{RWLocker: rl, Ptr: v}
	}
	return &JsVar[T]{RWLocker: rwlocker{l}, Ptr: v}
}

func isNilUI(ui jaws.UI) (yes bool) {
	if yes = (ui == nil); !yes {
		rv := reflect.ValueOf(ui)
		yes = rv.Kind() == reflect.Pointer && rv.IsNil()
	}
	return
}

// JsVar binds a [JsVar] to a named JavaScript variable.
//
// You can also pass a [JsVarMaker] instead of a [JsVar].
func (rw RequestWriter) JsVar(jsvarname string, jsvar any, params ...any) (err error) {
	if _, err = validateJsVarName([]any{jsvarname}); err == nil {
		if jvm, ok := jsvar.(JsVarMaker); ok {
			jsvar, err = jvm.JawsMakeJsVar(rw.Request)
		}
		if err == nil {
			err = ErrJsVarArgumentType
			if ui, ok := jsvar.(jaws.UI); ok {
				if !isNilUI(ui) {
					var newparams []any
					newparams = append(newparams, jsvarname)
					newparams = append(newparams, params...)
					err = rw.UI(ui, newparams...)
				}
			}
		}
	}
	return
}
