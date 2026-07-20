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
	// When an accepted update is broadcast, the JsVar marshals value before
	// releasing that write lock. Custom marshaling callbacks reachable from value,
	// including MarshalJSON and MarshalText, must not acquire the same locker or
	// re-enter the JsVar.
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

// JsVarCheck validates a tentative browser update to a [JsVar].
//
// The value contains the complete tentative state, and jsPath is the
// browser-supplied jq path used for the update. The path is passed through
// unchanged: jq accepts equivalent noncanonical spellings, including empty
// components, and both "" and "." address the root. Use jsPath as an inspection
// hint, not as an authorization key; implement [PathSetter] to allow-list paths.
// A nil error accepts the update. A non-nil error rejects it atomically and is
// returned unchanged, except that an error matching [ErrJsVarTooLarge] is
// returned as that sentinel after cancelling the associated request, when one
// is present. A panic rejects the update before continuing unchanged.
//
// The check validates tentative Go state only. A broadcast carries the decoded
// browser value rather than reading the stored path back, so jq conversions or
// ignored map-to-struct entries can make the peer value differ from the state
// inspected here. Use [PathSetter] when peer-visible input also needs validation.
//
// The check runs while the locker passed to [NewJsVar] is write-locked. It may
// inspect or marshal value, but it must not mutate it, acquire that locker,
// re-enter the JsVar, call a jq setter on the value, or retain references into a
// rejected tentative value. Any custom marshaling callback it invokes, including
// MarshalJSON or MarshalText, has the same restrictions. The check must not
// return or wrap [jaws.ErrEventUnhandled], which has handler-dispatch semantics.
type JsVarCheck[T any] func(value *T, jsPath string) error

// JSONSizeCheck returns a check that limits the JSON encoding of value.
//
// The check accepts an encoding whose length is exactly maxBytes and rejects a
// larger encoding with [ErrJsVarTooLarge]. A marshaling failure also matches
// ErrJsVarTooLarge and retains the marshaling error in its error chain. A
// non-positive maxBytes disables checking and returns nil.
//
// JSONSizeCheck marshals the complete value after every tentative change. Its
// time and allocation cost depend on the whole value and its marshaling behavior;
// map-key sorting and custom marshalers can add further cost.
// It bounds the encoding, not Go heap memory: custom JSON marshalers, omitted
// fields, aliases, slice capacity, and object overhead may make the two differ.
// Use a domain-specific [JsVarCheck] unless the JSON representation faithfully
// includes all state a client can grow.
func JSONSizeCheck[T any](maxBytes int) (check JsVarCheck[T]) {
	if maxBytes > 0 {
		check = func(value *T, _ string) (err error) {
			var data []byte
			if data, err = json.Marshal(value); err == nil {
				if len(data) > maxBytes {
					err = fmt.Errorf("%w: serialized size %d exceeds maximum %d", ErrJsVarTooLarge, len(data), maxBytes)
				}
			} else {
				err = fmt.Errorf("%w: cannot serialize value: %w", ErrJsVarTooLarge, err)
			}
			return
		}
	}
	return
}

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
// When bindings sharing a name also share a mutable backing object graph, a
// browser write applies to that shared state once per binding. Those bindings
// must use the same locker. Do not expose that state through several
// simultaneously rendered bindings when its [PathSetter] is not idempotent (for
// example one that appends).
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
// Rendering and write broadcasts invoke JSON marshalers while the locker passed
// to [NewJsVar] is held. This protects values retained by either the generic jq
// setter or a [PathSetter] from concurrent users of the same backing state.
// Custom marshaling callbacks reached in either case, including MarshalJSON and
// MarshalText, must not acquire that locker or re-enter the JsVar.
//
// A JsVar must not be copied after first use.
//
// SECURITY: a JsVar is client-writable. Incoming browser "set" messages are
// applied by path to the bound value. If the bound value implements [PathSetter],
// its JawsSetPath validates and applies the change. Otherwise the generic path
// setter ([github.com/linkdata/jq.Set]) can set any exported field — matched by its
// json tag, or by the Go field name when its json tag has no explicit name (a
// json:"-" tag is never writable) — and append to slices one element per
// message.
//
// There is no default cumulative size bound. Set [JsVar.ClientCheck] before first
// use to validate each tentative generic browser update. [JSONSizeCheck] provides
// an exact serialized-size limit. Every client-writable binding that can mutate
// the same backing object graph must share synchronization and use an equivalent
// checking policy. An unchecked or less restrictive binding can otherwise commit
// first; another binding then sees an unchanged value and does not run its check.
// A ClientCheck does not run for rendering, programmatic writes, invalid or
// unchanged writes, or values implementing PathSetter. It is an acceptance gate,
// not a monitor that proves the current value always satisfies an invariant.
//
// A size check does not prevent a client from setting individual exported fields.
// When only some fields or paths should be client-writable, implement [PathSetter]
// on the bound value to allow-list paths and bound lengths.
//
// Rejecting a browser write rolls back Go state without changing the value that
// the browser already assigned before sending it. Except for ErrJsVarTooLarge,
// which aborts the associated request when present, the application must
// resynchronize the browser if it requires immediate convergence after
// rejection.
// See the Node type in github.com/linkdata/jawstree for an example that restricts
// client writes to a single boolean field.
type JsVar[T any] struct {
	bind.RWLocker
	Ptr         *T            // bound Go value
	ClientCheck JsVarCheck[T] // optional check for generic browser writes; configure before first use
	setMu       sync.Mutex    // serializes each mutation with its broadcast
	dirtyTag    any           // current dirty tag, set during render; read via JawsGetTag
}

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

// setPathLocked applies the mutation and must be called with the write lock held.
func (jsvar *JsVar[T]) setPathLocked(elem *jaws.Element, jsPath string, value any, clientWrite bool) (changed, checkRejected, pathSetter bool, err error) {
	if jsvar.Ptr == nil {
		err = jq.ErrInvalidReceiver
	} else if ps, ok := any(jsvar.Ptr).(PathSetter); ok {
		pathSetter = true
		err = ps.JawsSetPath(elem, jsPath, value)
		if err == nil {
			changed = true
		}
	} else if clientWrite && jsvar.ClientCheck != nil {
		checkCalled := false
		changed, err = jq.SetChecked(jsvar.Ptr, jsPath, value, func() error {
			checkCalled = true
			return jsvar.ClientCheck(jsvar.Ptr, jsPath)
		})
		checkRejected = checkCalled && err != nil
	} else {
		changed, err = jq.Set(jsvar.Ptr, jsPath, value)
	}
	if err == nil && !changed && !clientWrite {
		err = jaws.ErrValueUnchanged
	}
	return
}

// setPathAndMarshal applies a mutation and prepares any broadcast payload while
// the bound value remains locked. A setter may retain a composite value directly
// in Ptr, so value can alias the shared state after a successful set.
func (jsvar *JsVar[T]) setPathAndMarshal(elem *jaws.Element, jsPath string, value any, clientWrite bool) (dirtyTag any, data []byte, changed, checkRejected, pathSetter bool, err error) {
	jsvar.Lock()
	defer jsvar.Unlock()
	if changed, checkRejected, pathSetter, err = jsvar.setPathLocked(elem, jsPath, value, clientWrite); err == nil && changed {
		dirtyTag = jsvar.dirtyTag
		if elem != nil && dirtyTag != nil {
			data, err = json.Marshal(value)
		}
	}
	return
}

func (jsvar *JsVar[T]) setPathLock(elem *jaws.Element, jsPath string, value any, clientWrite bool) (broadcasted, checkRejected, pathSetter bool, err error) {
	jsvar.setMu.Lock()
	defer jsvar.setMu.Unlock()

	var dirtyTag any
	var data []byte
	var changed bool
	dirtyTag, data, changed, checkRejected, pathSetter, err = jsvar.setPathAndMarshal(elem, jsPath, value, clientWrite)
	// dirtyTag is assigned only in JawsRender, so a set before the first render leaves
	// it nil. A what.Set with a nil Dest would target every element, and there is
	// nothing to update yet because the initial render carries the value in its
	// data-jawsdata attribute, so the broadcast is skipped in that case.
	//
	// The broadcast carries the caller's requested value, not the value actually
	// stored. If a PathSetter transforms the input (e.g. clamps a number), the
	// stored Go value and the value seen by peers can differ; the stored value is what
	// JawsGet returns. Reject noncanonical input or arrange reconciliation after the
	// PathSetter callback if peers must observe the transformed value.
	if err == nil && changed && elem != nil && dirtyTag != nil {
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
	var checkRejected bool
	var pathSetter bool
	broadcasted, checkRejected, pathSetter, err = jsvar.setPathLock(elem, jsPath, value, clientWrite)
	if clientWrite && pathSetter && err != nil && errors.Is(err, jaws.ErrValueUnchanged) {
		err = nil
	}
	if checkRejected && errors.Is(err, ErrJsVarTooLarge) {
		cause := err
		err = ErrJsVarTooLarge
		if elem != nil && elem.Request != nil {
			elem.Request.Cancel(cause)
		}
	}
	if err == nil && broadcasted {
		if sp, ok := any(jsvar.Ptr).(SetPather); ok {
			sp.JawsPathSet(elem, jsPath, value)
		}
	}
	return
}

// JawsSetPath sets the value at jsPath and broadcasts the change when possible.
// It is a programmatic server-side write, so it does not invoke
// [JsVar.ClientCheck].
//
// A nil elem changes the bound value without broadcasting. A set before this
// JsVar has acquired a dirty tag from rendering also produces no broadcast; its
// initial render seeds the value via the data-jawsdata attribute.
//
// The broadcast reaches matching active requests only. It is not replayed to a
// page between its initial render and its broadcast subscription; see [JsVar]
// for the synchronization model.
//
// When a write produces a broadcast, value is marshaled while the application
// locker is held because either the generic jq setter or a [PathSetter] may
// retain aliases into value. Custom marshaling callbacks reachable from value,
// including MarshalJSON and MarshalText, must not acquire that locker or re-enter
// the JsVar.
func (jsvar *JsVar[T]) JawsSetPath(elem *jaws.Element, jsPath string, value any) (err error) {
	return jsvar.setPath(elem, jsPath, value, false)
}

// JawsSet replaces the root value and broadcasts the change.
//
// It has the same delivery and marshaling semantics as [JsVar.JawsSetPath].
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
	jsvar.Unlock()

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

// JawsInput applies a browser-side JavaScript variable update.
//
// For a generic path write, a non-nil [JsVar.ClientCheck] validates the complete
// tentative state before it is committed. The check runs while the JsVar write
// lock is held. It must only inspect value: it must not mutate it, re-enter this
// JsVar, call a jq setter on it, or retain references into a rejected tentative
// value. An error rejects the write atomically, and a panic rolls the write back
// before it propagates.
//
// A ClientCheck error matching [ErrJsVarTooLarge] aborts the associated request,
// when elem has one, after the JsVar locks have been released. Other check errors
// reject the write without aborting a request. ClientCheck is not invoked for
// invalid or unchanged writes, values implementing [PathSetter], or
// programmatic [JsVar.JawsSetPath] calls.
//
// ClientCheck validates the tentative Go state. An accepted broadcast still
// carries the decoded browser value, which may differ after jq conversion or
// map-to-struct field selection; see [JsVarCheck].
func (jsvar *JsVar[T]) JawsInput(elem *jaws.Element, value string) (err error) {
	err = jaws.ErrEventUnhandled
	if jsPath, jsValue, found := strings.Cut(value, "="); found {
		var v any
		if err = json.Unmarshal([]byte(jsValue), &v); err == nil {
			err = jsvar.setPath(elem, jsPath, v, true)
		}
	}
	return
}

// NewJsVar creates a JsVar over v protected by l.
//
// The locker l must be non-nil and must remain valid for the lifetime of the JsVar.
// The pointer v may be nil; reads then return the zero value, rendering omits the
// initial data, and writes return [github.com/linkdata/jq.ErrInvalidReceiver].
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
