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
		// A JsVar name is used as a property key on the browser window: jaws.js
		// reads and writes window[name]. Assigning window["__proto__"] mutates the
		// window's prototype instead of setting a normal property, so it is reserved.
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
//
// JawsMakeJsVar must return a fresh [IsJsVar] for each request. The returned
// bindings may share synchronized backing state, but the bindings themselves
// must not be shared between requests.
type JsVarMaker interface {
	JawsMakeJsVar(rq *jaws.Request) (value IsJsVar, err error)
}

var (
	_ IsJsVar          = &JsVar[int]{}
	_ bind.Setter[int] = &JsVar[int]{}
)

// JsVar binds a Go value to a named JavaScript variable in the browser.
//
// A JsVar is request-scoped and must not be rendered by more than one
// [jaws.Request]. Construct a fresh JsVar for each request, either directly
// while rendering or through [JsVarMaker]. Distinct JsVar values may use the
// same locker and Ptr to expose synchronized application state to multiple
// requests.
//
// JsVar is intended for JSON-marshalable state shared with application
// JavaScript. The browser binding reads and writes the window property named
// when the JsVar is rendered. Existing application variables are therefore
// valid bindings. Do not use a browser-owned property such as window.name, or a
// global owned by unrelated code.
//
// Multiple bindings may share a name. The name is a single browser window
// property, and a browser-initiated write to it is delivered to every live
// binding of that name; a removed binding stops receiving writes. This lets a
// subtree re-render replace a binding, lets several requests expose the same
// application-owned global, and lets one browser value fan out to several
// independent Go bindings.
//
// When bindings sharing a name also share backing state (the same locker and
// Ptr), a browser write applies to that shared value once per binding. Do not
// expose one Ptr through several simultaneously rendered bindings when its
// [PathSetter] is not idempotent (for example one that appends).
//
// Unlike most JaWS UI values, a JsVar is a bidirectional channel and does not
// imply that the Go value is always authoritative. Browser and Go updates may
// each carry a complete value or individual paths. Any desired ownership,
// conflict, or merge policy is the application's responsibility.
//
// When Ptr is non-nil, [JsVar.JawsRender] serializes a snapshot of the bound Go
// value to initialize the browser variable. A browser call to the JavaScript
// jawsVar function sends only while its WebSocket is open; an earlier call is
// not queued for later transmission. [JsVar.JawsSet] and [JsVar.JawsSetPath]
// broadcast only to matching active requests, so an update is not replayed to a
// page between its initial render and its broadcast subscription. Applications
// that require the two sides to converge after either can change the value
// during that interval must reconcile it explicitly.
//
// It is safe for concurrent use when the locker passed to [NewJsVar] is safe
// for concurrent use. Concurrent writes are applied one at a time. Any
// broadcasts they produce preserve the order in which the writes modify the
// bound value. This concurrency guarantee does not permit one JsVar to be
// shared between requests.
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
// bound across many writes, a non-[PathSetter] value is size-accounted after each
// browser write and the [jaws.Request] is aborted with [ErrJsVarTooLarge] on the
// first write that pushes the serialized value past [MaxClientJsVarBytes] (an
// over-cap value present at render is likewise rejected). The cap does not prevent
// a client from setting individual exported fields, so when only some fields/paths
// should be client-writable, implement [PathSetter] on the bound value to
// allow-list paths and bound lengths.
// See jawstree's Node for an example that restricts client writes to a single
// boolean field.
type JsVar[T any] struct {
	bind.RWLocker
	Ptr       *T         // bound Go value
	setMu     sync.Mutex // serializes each mutation with its broadcast
	dirtyTag  any        // current dirty tag, set during render; read via JawsGetTag
	jsonBytes int        // running serialized-size estimate of Ptr; guarded by the write lock, maintained for client writes
}

// MaxClientJsVarBytes bounds the JSON-serialized size of a client-writable [JsVar]
// whose bound value does not implement [PathSetter].
//
// Without it, a hostile browser could grow such a JsVar's server-side state without
// bound across many writes (each single write is already bounded by the WebSocket
// read limit). The serialized size is accounted after each browser write and a value
// larger than the cap aborts the [jaws.Request] with [ErrJsVarTooLarge] on the first
// over-cap write; an over-cap value present at render is rejected there instead.
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

// capApplies reports whether client-write size accounting is active for this
// JsVar: [MaxClientJsVarBytes] is positive and the bound value does not implement
// [PathSetter]. A PathSetter enforces its own bounds and is exempt.
//
// The result depends only on the package global and the static type of Ptr, so it
// needs no lock.
func (jsvar *JsVar[T]) capApplies() bool {
	if MaxClientJsVarBytes <= 0 {
		return false
	}
	_, isPathSetter := any(jsvar.Ptr).(PathSetter)
	return !isPathSetter
}

// exceedsClientJsVarCap reports whether n bytes exceeds the configured
// [MaxClientJsVarBytes] for this JsVar. Values implementing [PathSetter] enforce
// their own bounds, so the cap does not apply to them, and a non-positive
// MaxClientJsVarBytes disables it.
func (jsvar *JsVar[T]) exceedsClientJsVarCap(n int) bool {
	return jsvar.capApplies() && n > MaxClientJsVarBytes
}

// accountClientWrite folds the growth of the latest client mutation into the
// running serialized-size estimate and reports the resulting size and whether it
// now exceeds [MaxClientJsVarBytes].
//
// valueBytes is the marshaled length of the value just applied. Because
// [github.com/linkdata/jq.Set] only grows the bound value by appending a single
// slice element (never creating intermediate structure), the actual serialized
// growth of any one write is at most valueBytes plus one JSON separator, so adding
// that upper bound keeps the estimate at or above the true size. When the estimate
// crosses the cap the true size is confirmed by a single marshal of Ptr, which
// also reclaims any over-count so a value legitimately under the cap does not force
// a marshal on every later write. It returns overCap only for a size confirmed by
// that marshal, never on the estimate alone.
//
// It is called only when [JsVar.capApplies] is true and acquires the write lock.
func (jsvar *JsVar[T]) accountClientWrite(valueBytes int) (size int, overCap bool) {
	jsvar.Lock()
	defer jsvar.Unlock()
	jsvar.jsonBytes += valueBytes + 1
	if jsvar.jsonBytes > MaxClientJsVarBytes {
		if data, err := json.Marshal(jsvar.Ptr); err == nil {
			jsvar.jsonBytes = len(data)
			overCap = jsvar.jsonBytes > MaxClientJsVarBytes
		}
	}
	size = jsvar.jsonBytes
	return
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

func (jsvar *JsVar[T]) setPathLock(elem *jaws.Element, jsPath string, value any, clientWrite bool) (broadcasted bool, err error) {
	jsvar.setMu.Lock()
	defer jsvar.setMu.Unlock()
	var dirtyTag any
	if dirtyTag, err = jsvar.setPathAndGetTag(elem, jsPath, value); err != nil {
		return
	}

	// Marshal the applied value once, outside the caller-provided lock: value is the
	// caller-owned argument (not read from Ptr), and jaws.Broadcast can block on the
	// broadcast channel under backpressure. The private setMu remains held so
	// concurrent setters cannot apply a later mutation before this write's size
	// accounting and broadcast complete. Code sharing the caller-provided locker is
	// therefore not stalled by transport backpressure.
	//
	// dirtyTag is assigned only in JawsRender, so a set before the first render
	// leaves it nil. A what.Set with a nil Dest would target every element, and there
	// is nothing to update yet because the initial render carries the value in its
	// data-jawsdata attribute, so the broadcast is skipped in that case. A client
	// write still needs the marshaled length to enforce MaxClientJsVarBytes, so the
	// value is marshaled whenever either the broadcast or the size accounting needs it.
	willBroadcast := elem != nil && dirtyTag != nil
	accountSize := clientWrite && jsvar.capApplies()
	var data []byte
	if willBroadcast || accountSize {
		if data, err = json.Marshal(value); err != nil {
			return
		}
	}

	// Enforce MaxClientJsVarBytes on the accumulated growth from untrusted browser
	// writes, aborting the request on the first over-cap write rather than relying on
	// a later render that a normally rendered JsVar never performs. Programmatic writes
	// (JawsSetPath) are trusted and not capped; see MaxClientJsVarBytes.
	if accountSize {
		if size, overCap := jsvar.accountClientWrite(len(data)); overCap {
			err = ErrJsVarTooLarge
			if elem != nil {
				elem.Request.Cancel(fmt.Errorf("jaws: jsvar serialized size %d exceeds MaxClientJsVarBytes (%d)", size, MaxClientJsVarBytes))
			}
			return
		}
	}

	// The broadcast carries the caller's requested value, not the value actually
	// stored. If a PathSetter coerces or rejects the input (e.g. clamps a number), the
	// stored Go value and the value seen by peers can differ; the stored value is what
	// JawsGet returns. Re-broadcast from Ptr inside a PathSetter if peers must observe
	// the coerced value.
	if willBroadcast {
		elem.Jaws.Broadcast(wire.Message{
			Dest: dirtyTag,
			What: what.Set,
			Data: jsPath + "=" + string(data),
		})
		broadcasted = true
	}
	return
}

func (jsvar *JsVar[T]) setPath(elem *jaws.Element, jsPath string, value any, clientWrite bool) (err error) {
	// jsPath is written verbatim into a what.Set wire frame (only the value side
	// is JSON-encoded). The client splits frames on '\n', fields on '\t', and the
	// JsVar payload at the first '='. Reject any path carrying those protocol
	// bytes before applying or broadcasting it: they either corrupt the frame or
	// make peers parse the value as invalid JSON.
	if strings.ContainsAny(jsPath, "\t\n\r=") {
		return ErrIllegalJsVarPath
	}
	var broadcasted bool
	if broadcasted, err = jsvar.setPathLock(elem, jsPath, value, clientWrite); err == nil && broadcasted {
		if sp, ok := any(jsvar.Ptr).(SetPather); ok {
			sp.JawsPathSet(elem, jsPath, value)
		}
	}
	return
}

// JawsSetPath sets the value at jsPath and broadcasts the change when possible.
// It is a programmatic (server-side, trusted) write and is not size-capped at
// the write boundary; see [MaxClientJsVarBytes] for the browser-write cap.
//
// A nil elem changes the bound value without broadcasting. A set before this
// JsVar has acquired a dirty tag from rendering also produces no broadcast; its
// initial render seeds the value via the data-jawsdata attribute.
//
// The broadcast reaches matching active requests only. It is not replayed to a
// page between its initial render and its broadcast subscription; see [JsVar]
// for the synchronization model.
func (jsvar *JsVar[T]) JawsSetPath(elem *jaws.Element, jsPath string, value any) (err error) {
	return jsvar.setPath(elem, jsPath, value, false)
}

// JawsSet replaces the root value and broadcasts the change.
//
// It has the same delivery semantics as [JsVar.JawsSetPath].
func (jsvar *JsVar[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	return jsvar.JawsSetPath(elem, "", value)
}

// JawsRender writes the hidden element that seeds and routes the JavaScript variable.
//
// params[0] must be a valid JsVar name. Otherwise, JawsRender returns
// [ErrIllegalJsVarName] without writing markup.
//
// A name may be bound by more than one live binding; see [JsVar] for how a
// browser write is delivered to every live binding of the name.
//
// The serialized value is a render-time snapshot. See [JsVar] for the
// synchronization semantics between rendering and the WebSocket subscription.
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
	// Seed the client-write size accounting with the rendered size so subsequent
	// browser writes measure growth from the value the page actually received.
	jsvar.jsonBytes = len(data)
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
//
// Dirtying a JsVar therefore does not resend its root value. Use [JsVar.JawsSet]
// or [JsVar.JawsSetPath], together with the application's synchronization
// policy, to send changes.
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
// A single incoming message is already bounded by the connection's WebSocket read
// limit (SetReadLimit in the request handler). To also bound cumulative growth, a
// non-[PathSetter] value is size-accounted after each applied write and the request
// is aborted with [ErrJsVarTooLarge] on the first write that pushes the serialized
// value past [MaxClientJsVarBytes]. The accounting folds an upper bound on each
// write's growth into a running total and marshals the whole value only when that
// total crosses the cap, so an append flood stays O(n) rather than O(n^2).
func (jsvar *JsVar[T]) JawsInput(elem *jaws.Element, value string) (err error) {
	err = jaws.ErrEventUnhandled
	if jsPath, jsValue, found := strings.Cut(value, "="); found {
		var v any
		if err = json.Unmarshal([]byte(jsValue), &v); err == nil {
			err = elideErrValueUnchanged(jsvar.setPath(elem, jsPath, v, true))
		}
	}
	return
}

// NewJsVar creates a JsVar over v protected by l.
//
// The locker l must be non-nil and must remain valid for the lifetime of the JsVar.
// Create a fresh JsVar for each request; l and v may be shared by distinct
// request-scoped JsVar values. Use [JsVarMaker] when construction depends on the
// current request or the maker is stored in shared handler data.
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
// jsvarName identifies a property on the browser window. It should be owned by
// the application because the binding initializes and updates its value.
//
// See [JsVar] for the bidirectional binding and synchronization semantics,
// including how a name shared by several live bindings is routed.
//
// It returns [ErrIllegalJsVarName] if jsvarName is invalid or reserved.
//
// A directly supplied [JsVar] must be scoped to rw.Request. You can instead pass
// a [JsVarMaker], which is useful when the maker is stored in handler or template
// data shared by multiple requests.
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
