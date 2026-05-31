package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/jq"
)

var jsVarNameRx = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func validateJsVarName(value []any) (name string, err error) {
	if len(value) > 0 {
		var ok bool
		if name, ok = value[0].(string); !ok {
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
	// JawsSetPath should set the JSON object member identified by jsPath to the given value.
	//
	// If the member is already the given value, it should return [jaws.ErrValueUnchanged].
	JawsSetPath(elem *jaws.Element, jsPath string, value any) (err error)
}

// SetPather is notified after a nested JSON path value has been set and
// broadcast.
type SetPather interface {
	// JawsPathSet notifies that a JSON object member identified by jsPath has been set
	// to the given value and the change has been queued for broadcast.
	JawsPathSet(elem *jaws.Element, jsPath string, value any)
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
	JawsMakeJsVar(rq *jaws.Request) (value IsJsVar, err error)
}

var (
	_ IsJsVar          = &JsVar[int]{}
	_ bind.Setter[int] = &JsVar[int]{}
)

// JsVar binds a Go value to a named JavaScript variable in the browser.
//
// It is safe for concurrent use when the locker passed to [NewJsVar] is safe
// for concurrent use.
//
// SECURITY: a JsVar is client-writable. Incoming browser "set" messages are
// applied by path to the bound value. If the bound value implements [PathSetter]
// its JawsSetPath validates and applies the change; otherwise the change is
// applied by the generic path setter ([github.com/linkdata/jq.Set]), which will
// set any json-tagged field and append to slices one element per message. The size
// of any single client write is bounded by the WebSocket read limit; to also stop a
// hostile client growing server state without bound across many writes, a
// non-[PathSetter] value whose serialized size exceeds [MaxClientJsVarBytes] aborts
// the [Request] when it is next rendered ([ErrJsVarTooLarge]). The cap does not
// prevent a client from setting individual json-tagged fields, so when only some
// fields/paths should be client-writable, implement [PathSetter] on the bound value
// to allow-list paths and bound lengths. See jawstree's Node for an example that
// restricts client writes to a single boolean field.
type JsVar[T any] struct {
	bind.RWLocker
	Ptr      *T  // bound Go value
	dirtyTag any // current dirty tag, set during render; read via JawsGetTag
}

// MaxClientJsVarBytes bounds the JSON-serialized size of a client-writable [JsVar]
// whose bound value does not implement [PathSetter].
//
// Set it before serving requests; a value <= 0 disables the cap. Values that
// implement [PathSetter] enforce their own bounds and are exempt.
//
// Such a JsVar is written by the generic path setter, which a hostile browser could
// use to grow server-side state without bound across many writes (each individual
// write is already bounded by the WebSocket read limit). When the value is
// serialized for the browser in [JsVar.JawsRender], a value larger than this many
// bytes aborts the [Request] ([ErrJsVarTooLarge]); the bound value is never
// marshaled solely to measure it, which would turn an append flood into O(n^2) work.
var MaxClientJsVarBytes = 1 << 20 // 1 MiB

// JawsGetPath returns the value at jsPath, logging lookup errors on elem when possible.
func (jsvar *JsVar[T]) JawsGetPath(elem *jaws.Element, jsPath string) (value any) {
	jsvar.RLock()
	defer jsvar.RUnlock()
	var err error
	value, err = jq.Get(jsvar.Ptr, jsPath)
	if elem != nil {
		_ = elem.Jaws.Log(err)
	}
	return
}

// JawsGet returns the bound value.
func (jsvar *JsVar[T]) JawsGet(elem *jaws.Element) (value T) {
	jsvar.RLock()
	defer jsvar.RUnlock()
	if jsvar.Ptr != nil {
		value = *jsvar.Ptr
	}
	return
}

// exceedsClientJsVarCap reports whether n bytes exceeds the configured
// [MaxClientJsVarBytes] for this JsVar. Values implementing [PathSetter] enforce
// their own bounds, so the cap does not apply to them, and a non-positive
// MaxClientJsVarBytes disables it.
func (jsvar *JsVar[T]) exceedsClientJsVarCap(n int) bool {
	if MaxClientJsVarBytes <= 0 || n <= MaxClientJsVarBytes {
		return false
	}
	_, isPathSetter := any(jsvar.Ptr).(PathSetter)
	return !isPathSetter
}

// setPathLocked applies the mutation and must be called with the write lock held.
func (jsvar *JsVar[T]) setPathLocked(elem *jaws.Element, jsPath string, value any) (err error) {
	if ps, ok := ((any)(jsvar.Ptr).(PathSetter)); ok {
		err = ps.JawsSetPath(elem, jsPath, value)
	} else {
		var changed bool
		if changed, err = jq.Set(jsvar.Ptr, jsPath, value); err == nil && !changed {
			err = jaws.ErrValueUnchanged
		}
	}
	return
}

func (jsvar *JsVar[T]) setPathLock(elem *jaws.Element, jsPath string, value any) (err error) {
	jsvar.Lock()
	err = jsvar.setPathLocked(elem, jsPath, value)
	dirtyTag := jsvar.dirtyTag
	jsvar.Unlock()
	// Marshal and broadcast outside the lock: value is the caller-owned argument
	// (not read from Ptr), and jaws.Broadcast can block on the broadcast channel
	// under backpressure. Holding the lock across that send would needlessly
	// serialize concurrent setters and stall any code sharing the locker. This
	// mirrors bind.binder, which mutates under the lock and runs side effects
	// after releasing it.
	//
	// Note that the broadcast carries the caller's requested value, not the value
	// actually stored. If a PathSetter coerces or rejects the input (e.g. clamps a
	// number), the stored server state and the value seen by peers can differ; the
	// authoritative state is what JawsGet returns. Re-broadcast from Ptr inside a
	// PathSetter if peers must observe the coerced value.
	if err == nil && elem != nil {
		var data []byte
		if data, err = json.Marshal(value); err == nil {
			elem.Jaws.Broadcast(wire.Message{
				Dest: dirtyTag,
				What: what.Set,
				Data: jsPath + "=" + string(data),
			})
		}
	}
	return
}

func (jsvar *JsVar[T]) setPath(elem *jaws.Element, jsPath string, value any) (err error) {
	// jsPath is written verbatim into a what.Set wire frame (only the value side
	// is JSON-encoded). The client splits frames on '\n' and fields on '\t', so a
	// path containing a tab, newline or carriage return could corrupt the frame or
	// inject fabricated orders into every peer browser sharing this JsVar. Reject
	// such a path before applying or broadcasting it: these bytes never occur in
	// the trusted client's own dotted-identifier sender and have no valid meaning
	// in a jq path. This mirrors the framing defense jaws.JsCall applies to the
	// sibling verbatim what.Call payload.
	if strings.ContainsAny(jsPath, "\t\n\r") {
		return ErrIllegalJsVarPath
	}
	if err = jsvar.setPathLock(elem, jsPath, value); err == nil {
		if sp, ok := ((any)(jsvar.Ptr).(SetPather)); ok {
			sp.JawsPathSet(elem, jsPath, value)
		}
	}
	return
}

// JawsSetPath sets the value at jsPath and broadcasts the change. It is a
// programmatic (server-side, trusted) write and is not size-capped at the write
// boundary; see [MaxClientJsVarBytes] for the browser-write cap.
func (jsvar *JsVar[T]) JawsSetPath(elem *jaws.Element, jsPath string, value any) (err error) {
	return jsvar.setPath(elem, jsPath, value)
}

// JawsSet replaces the root value and broadcasts the change.
func (jsvar *JsVar[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	return jsvar.JawsSetPath(elem, "", value)
}

// JawsRender writes the hidden element that seeds and routes the JavaScript variable.
//
// The write lock is held only while deriving the dirty tag from the bound value
// (via [jaws.Element.ApplyGetter]) and marshaling it, so the marshaled Ptr stays
// consistent with that tag even if another request sharing this JsVar sets it
// concurrently. The lock is released before [jaws.Element.ApplyParams] and, crucially,
// before writing to w: holding the value lock across a network write would let a
// slow client stall every goroutine sharing the locker. While the lock is held the
// bound value's [tag.TagGetter.JawsGetTag] and [jaws.InitHandler.JawsInit]
// callbacks run, so they must not re-enter this JsVar (e.g. call JawsGet or
// JawsSet on it), which would self-deadlock the non-reentrant lock.
func (jsvar *JsVar[T]) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	var jsvarName string
	var data []byte

	jsvar.Lock()
	if jsvar.dirtyTag, getterAttrs, err = elem.ApplyGetter(jsvar.Ptr); err == nil {
		elem.AddHandlers(jsvar)
		if jsvarName, err = validateJsVarName(params); err == nil && jsvar.Ptr != nil {
			data, err = json.Marshal(jsvar.Ptr)
		}
	}
	jsvar.Unlock()

	// After the fact: if the value has grown past the cap (e.g. via accumulated
	// client writes), abort the request rather than emit an oversized payload. This
	// reuses the marshal above; the value is never marshaled solely to measure it.
	if err == nil && jsvar.exceedsClientJsVarCap(len(data)) {
		err = ErrJsVarTooLarge
		elem.Request.Cancel(fmt.Errorf("jaws: jsvar serialized size %d exceeds MaxClientJsVarBytes (%d)", len(data), MaxClientJsVarBytes))
	}

	if err == nil {
		attrs := append(elem.ApplyParams(params[1:]), getterAttrs...)
		var b []byte
		b = append(b, "\n<div id="...)
		b = elem.Jid().AppendQuote(b)
		b = htmlio.AppendAttr(b, "data-jawsname", jsvarName)
		if data != nil {
			b = htmlio.AppendAttr(b, "data-jawsdata", string(data))
		}
		b = htmlio.AppendAttrs(b, attrs)
		b = append(b, " hidden></div>"...)
		_, err = w.Write(b)
	}
	return
}

// JawsGetTag returns the current dirty tag.
//
// It is safe for concurrent use. The tag.Context argument is ignored and may be nil.
func (jsvar *JsVar[T]) JawsGetTag(tag.Context) any {
	jsvar.RLock()
	defer jsvar.RUnlock()
	return jsvar.dirtyTag
}

// JawsUpdate is a no-op because updates are broadcast by path setters.
func (jsvar *JsVar[T]) JawsUpdate(elem *jaws.Element) {
	_ = elem // no-op for JsVar[T]
}

func elideErrValueUnchanged(err error) error {
	if errors.Is(err, jaws.ErrValueUnchanged) {
		return nil
	}
	return err
}

// JawsInput applies a browser-side JavaScript variable update.
//
// There is no per-write size check here: the size of any single incoming message
// is already bounded by the WebSocket read limit set on the connection (see
// SetReadLimit in the request handler). Cumulative growth of a non-[PathSetter]
// value past [MaxClientJsVarBytes] is caught after the fact in [JsVar.JawsRender],
// which avoids re-marshaling the bound value on every write (an append flood would
// otherwise be O(n^2)).
func (jsvar *JsVar[T]) JawsInput(elem *jaws.Element, value string) (err error) {
	err = jaws.ErrEventUnhandled
	if jsPath, jsValue, found := strings.Cut(value, "="); found {
		var v any
		if err = json.Unmarshal([]byte(jsValue), &v); err == nil {
			err = elideErrValueUnchanged(jsvar.setPath(elem, jsPath, v))
		}
	}
	return
}

// NewJsVar creates a JsVar over v protected by l.
//
// The locker l must be non-nil and must remain valid for the lifetime of the JsVar.
func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	return &JsVar[T]{RWLocker: bind.AsRWLocker(l), Ptr: v}
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
func (rw RequestWriter) JsVar(jsvarName string, jsvar any, params ...any) (err error) {
	if _, err = validateJsVarName([]any{jsvarName}); err == nil {
		if jvm, ok := jsvar.(JsVarMaker); ok {
			jsvar, err = jvm.JawsMakeJsVar(rw.Request)
		}
		if err == nil {
			err = ErrJsVarArgumentType
			if ui, ok := jsvar.(jaws.UI); ok {
				if !isNilUI(ui) {
					var newparams []any
					newparams = append(newparams, jsvarName)
					newparams = append(newparams, params...)
					err = rw.UI(ui, newparams...)
				}
			}
		}
	}
	return
}
