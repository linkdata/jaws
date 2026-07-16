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
			return
		}
		if name == "__proto__" {
			err = errIllegalJsVarName("reserved")
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
	//
	// When a [JsVar]'s bound value (Ptr) implements PathSetter, the JsVar
	// delegates to it while holding the JsVar write lock. Such an
	// implementation must not lock or unlock the JsVar, nor call its locked
	// accessors such as [JsVar.JawsGet] or [JsVar.JawsSet].
	//
	// If an implementation panics, the calling JsVar releases its write lock
	// before propagating the panic.
	JawsSetPath(elem *jaws.Element, jsPath string, value any) (err error)
}

// SetPather is notified after a nested JSON path value has been set and
// broadcast.
type SetPather interface {
	// JawsPathSet notifies that a JSON object member identified by jsPath has been set
	// to the given value and the change has been queued for broadcast.
	//
	// Unlike [PathSetter.JawsSetPath], a [JsVar] calls this after releasing
	// its lock, so locking the JsVar or calling its locked accessors is
	// allowed here.
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
// for concurrent use. Concurrent writes are applied one at a time. Any
// broadcasts they produce preserve the order in which the writes modify the
// bound value.
//
// A JsVar must not be copied after first use.
//
// SECURITY: a JsVar is client-writable. Incoming browser "set" messages are
// applied by path to the bound value. If the bound value implements [PathSetter]
// its JawsSetPath validates and applies the change; otherwise the change is
// applied by the generic path setter ([github.com/linkdata/jq.Set]), which will
// set any exported field — matched by its json tag, or by the Go field name when it
// has no json tag (a json:"-" tag is never writable) — and append to slices one
// element per message. The size of any single client write is bounded by the
// WebSocket read limit; to also stop a hostile client growing server state without
// bound across many writes, a non-[PathSetter] value whose serialized size exceeds
// [MaxClientJsVarBytes] aborts the [jaws.Request] when it is next rendered
// ([ErrJsVarTooLarge]). The cap does not prevent a client from setting individual
// exported fields, so when only some fields/paths should be client-writable,
// implement [PathSetter] on the bound value to allow-list paths and bound lengths.
// See jawstree's Node for an example that restricts client writes to a single
// boolean field.
type JsVar[T any] struct {
	bind.RWLocker
	Ptr      *T         // bound Go value
	setMu    sync.Mutex // serializes each mutation with its broadcast
	dirtyTag any        // current dirty tag, set during render; read via JawsGetTag
}

// MaxClientJsVarBytes bounds the JSON-serialized size of a client-writable [JsVar]
// whose bound value does not implement [PathSetter].
//
// Without it, a hostile browser could grow such a JsVar's server-side state without
// bound across many writes (each single write is already bounded by the WebSocket
// read limit). A value larger than the cap aborts the [jaws.Request] with
// [ErrJsVarTooLarge] when next rendered.
//
// Set it once before serving requests; a value <= 0 disables the cap, and values
// that implement [PathSetter] enforce their own bounds and are exempt. It is a
// plain package global read on the render path, so mutating it while requests are
// being served is a data race.
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
	if ps, ok := any(jsvar.Ptr).(PathSetter); ok {
		err = ps.JawsSetPath(elem, jsPath, value)
	} else {
		var changed bool
		if changed, err = jq.Set(jsvar.Ptr, jsPath, value); err == nil && !changed {
			err = jaws.ErrValueUnchanged
		}
	}
	return
}

func (jsvar *JsVar[T]) setPathAndGetTag(elem *jaws.Element, jsPath string, value any) (dirtyTag any, err error) {
	jsvar.Lock()
	defer jsvar.Unlock()
	err = jsvar.setPathLocked(elem, jsPath, value)
	dirtyTag = jsvar.dirtyTag
	return
}

func (jsvar *JsVar[T]) setPathLock(elem *jaws.Element, jsPath string, value any) (broadcasted bool, err error) {
	jsvar.setMu.Lock()
	defer jsvar.setMu.Unlock()
	dirtyTag, err := jsvar.setPathAndGetTag(elem, jsPath, value)
	// Marshal and broadcast outside the caller-provided lock: value is the
	// caller-owned argument (not read from Ptr), and jaws.Broadcast can block on
	// the broadcast channel under backpressure. The private setMu remains held so
	// concurrent setters cannot apply a later mutation before this broadcast is
	// queued. Code sharing the caller-provided locker is therefore not stalled by
	// transport backpressure.
	//
	// Note that the broadcast carries the caller's requested value, not the value
	// actually stored. If a PathSetter coerces or rejects the input (e.g. clamps a
	// number), the stored server state and the value seen by peers can differ; the
	// authoritative state is what JawsGet returns. Re-broadcast from Ptr inside a
	// PathSetter if peers must observe the coerced value.
	//
	// dirtyTag is assigned only in JawsRender, so a set before the first render
	// leaves it nil. Skip the broadcast in that case: a what.Set with a nil Dest
	// would target every element, and there is nothing to update yet because the
	// initial render carries the value in its data-jawsdata attribute.
	if err == nil && elem != nil && dirtyTag != nil {
		var data []byte
		if data, err = json.Marshal(value); err == nil {
			elem.Jaws.Broadcast(wire.Message{
				Dest: dirtyTag,
				What: what.Set,
				Data: jsPath + "=" + string(data),
			})
			broadcasted = true
		}
	}
	return
}

func (jsvar *JsVar[T]) setPath(elem *jaws.Element, jsPath string, value any) (err error) {
	// jsPath is written verbatim into a what.Set wire frame (only the value side
	// is JSON-encoded). The client splits frames on '\n', fields on '\t', and the
	// JsVar payload at the first '='. Reject any path carrying those protocol
	// bytes before applying or broadcasting it: they either corrupt the frame or
	// make peers parse the value as invalid JSON.
	if strings.ContainsAny(jsPath, "\t\n\r=") {
		return ErrIllegalJsVarPath
	}
	var broadcasted bool
	if broadcasted, err = jsvar.setPathLock(elem, jsPath, value); err == nil && broadcasted {
		if sp, ok := any(jsvar.Ptr).(SetPather); ok {
			sp.JawsPathSet(elem, jsPath, value)
		}
	}
	return
}

// JawsSetPath sets the value at jsPath and broadcasts the change. It is a
// programmatic (server-side, trusted) write and is not size-capped at the write
// boundary; see [MaxClientJsVarBytes] for the browser-write cap.
//
// A set before the element has been rendered produces no broadcast: the dirty
// tag does not exist yet and the initial render seeds the value via its
// data-jawsdata attribute.
func (jsvar *JsVar[T]) JawsSetPath(elem *jaws.Element, jsPath string, value any) (err error) {
	return jsvar.setPath(elem, jsPath, value)
}

// JawsSet replaces the root value and broadcasts the change.
//
// A set before the element has been rendered produces no broadcast: the dirty
// tag does not exist yet and the initial render seeds the value via its
// data-jawsdata attribute.
func (jsvar *JsVar[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	return jsvar.JawsSetPath(elem, "", value)
}

// JawsRender writes the hidden element that seeds and routes the JavaScript variable.
//
// The bound value's [tag.TagGetter.JawsGetTag] and [jaws.InitHandler.JawsInit]
// callbacks run while the JsVar write lock is held, so they must not re-enter this
// JsVar (for example call JawsGet or JawsSet on it), which would self-deadlock the
// non-reentrant lock.
func (jsvar *JsVar[T]) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	var jsvarName string
	var data []byte

	// Hold the write lock only while deriving the dirty tag (ApplyGetter) and
	// marshaling Ptr, so the marshaled value stays consistent with that tag even if
	// another request sharing this JsVar sets it concurrently. Release before
	// ApplyParams and, crucially, before writing to w: holding the value lock across a
	// network write would let a slow client stall every goroutine sharing the locker.
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
func (jsvar *JsVar[T]) JawsInput(elem *jaws.Element, value string) (err error) {
	// No per-write size check: a single incoming message is already bounded by the
	// connection's WebSocket read limit (SetReadLimit in the request handler), and
	// cumulative growth of a non-PathSetter value past MaxClientJsVarBytes is caught
	// after the fact in JawsRender, which avoids re-marshaling on every write (an
	// append flood would otherwise be O(n^2)).
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
					err = rw.NewUI(ui, newparams...)
				}
			}
		}
	}
	return
}
