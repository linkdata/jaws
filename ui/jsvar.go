package ui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
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

type PathSetter interface {
	// JawsSetPath should set the JSON object member identified by jspath to the given value.
	//
	// If the member is already the given value, it should return ErrValueUnchanged.
	JawsSetPath(elem *core.Element, jspath string, value any) (err error)
}

type SetPather interface {
	// JawsPathSet notifies that a JSON object member identified by jspath has been set
	// to the given value and the change has been queued for broadcast.
	JawsPathSet(elem *core.Element, jspath string, value any)
}

type IsJsVar interface {
	core.RWLocker
	core.UI
	core.EventHandler
	PathSetter
}

type JsVarMaker interface {
	JawsMakeJsVar(rq *core.Request) (v IsJsVar, err error)
}

var (
	_ IsJsVar          = &JsVar[int]{}
	_ core.Setter[int] = &JsVar[int]{}
)

type JsVar[T any] struct {
	core.RWLocker
	Ptr *T
	Tag any
}

func (ui *JsVar[T]) JawsGetPath(elem *core.Element, jspath string) (value any) {
	ui.RLock()
	defer ui.RUnlock()
	var err error
	value, err = jq.Get(ui.Ptr, jspath)
	if elem != nil {
		_ = elem.Jaws.Log(err)
	}
	return
}

func (ui *JsVar[T]) JawsGet(elem *core.Element) (value T) {
	ui.RLock()
	defer ui.RUnlock()
	if ui.Ptr != nil {
		value = *ui.Ptr
	}
	return
}

func (ui *JsVar[T]) setPathLocked(elem *core.Element, jspath string, value any) (err error) {
	if ps, ok := ((any)(ui.Ptr).(PathSetter)); ok {
		err = ps.JawsSetPath(elem, jspath, value)
	} else {
		var changed bool
		if changed, err = jq.Set(ui.Ptr, jspath, value); err == nil && !changed {
			err = core.ErrValueUnchanged
		}
	}
	if err == nil && elem != nil {
		var data []byte
		if data, err = json.Marshal(value); err == nil {
			elem.Jaws.Broadcast(core.Message{
				Dest: ui.Tag,
				What: what.Set,
				Data: jspath + "=" + string(data),
			})
		}
	}
	return
}

func (ui *JsVar[T]) setPathLock(elem *core.Element, jspath string, value any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	err = ui.setPathLocked(elem, jspath, value)
	return
}

func (ui *JsVar[T]) setPath(elem *core.Element, jspath string, value any) (err error) {
	if err = ui.setPathLock(elem, jspath, value); err == nil {
		if sp, ok := ((any)(ui.Ptr).(SetPather)); ok {
			sp.JawsPathSet(elem, jspath, value)
		}
	}
	return
}

func (ui *JsVar[T]) JawsSetPath(elem *core.Element, jspath string, value any) (err error) {
	return ui.setPath(elem, jspath, value)
}

func (ui *JsVar[T]) JawsSet(elem *core.Element, value T) (err error) {
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

func (ui *JsVar[T]) JawsRender(e *core.Element, w io.Writer, params []any) (err error) {
	ui.Lock()
	defer ui.Unlock()
	if ui.Tag, err = e.ApplyGetter(ui.Ptr); err == nil {
		var jsvarname string
		if jsvarname, err = validateJsVarName(params); err == nil {
			var data []byte
			if ui.Ptr != nil {
				data, err = json.Marshal(ui.Ptr)
			}
			if err == nil {
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
	}
	return
}

func (ui *JsVar[T]) JawsGetTag(rq *core.Request) any {
	return ui.Tag
}

func (ui *JsVar[T]) JawsUpdate(e *core.Element) {} // no-op for JsVar[T]

func elideErrValueUnchanged(err error) error {
	if err == core.ErrValueUnchanged {
		return nil
	}
	return err
}

func (ui *JsVar[T]) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
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

// NewJsVar creates a JsVar over v protected by l.
//
// The locker l must be non-nil and must remain valid for the lifetime of the JsVar.
func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	if rl, ok := l.(core.RWLocker); ok {
		return &JsVar[T]{RWLocker: rl, Ptr: v}
	}
	return &JsVar[T]{RWLocker: rwlocker{l}, Ptr: v}
}

// JsVar binds a JsVar[T] to a named Javascript variable.
//
// You can also pass a JsVarMaker instead of a JsVar[T].
func (rqw RequestWriter) JsVar(jsvarname string, jsvar any, params ...any) (err error) {
	if _, err = validateJsVarName([]any{jsvarname}); err == nil {
		if jvm, ok := jsvar.(JsVarMaker); ok {
			jsvar, err = jvm.JawsMakeJsVar(rqw.Request)
		}
		if err == nil {
			err = ErrJsVarArgumentType
			if ui, ok := jsvar.(core.UI); ok {
				var newparams []any
				newparams = append(newparams, jsvarname)
				newparams = append(newparams, params...)
				err = rqw.UI(ui, newparams...)
			}
		}
	}
	return
}
