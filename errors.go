package jaws

import (
	"errors"
	"fmt"
	"net/netip"
	"reflect"

	"github.com/linkdata/jaws/lib/key"
)

// ErrServeAlreadyRunning indicates the JaWS processing loop is already running.
var ErrServeAlreadyRunning = errors.New("serve loop already running")

// ErrValueUnchanged reports a successful no-op set: there was no error, but the
// underlying value already equaled the desired value.
//
// Setter-style implementations (the JawsSet / JawsSetPath methods in
// github.com/linkdata/jaws/lib/ui and github.com/linkdata/jawstree) return it,
// and callers test for it with [errors.Is]. It lives in this package so all
// implementations share one error identity.
var ErrValueUnchanged = errors.New("value unchanged")

// ErrRequestOverloaded indicates a [Request] was torn down because it could not keep
// up with the messages addressed to it.
//
// A Request is overloaded when its buffered broadcast channel or its internal
// event-call channel fills before it can drain them. Rather than silently dropping
// messages, which could leave the browser and backend in inconsistent and
// nonreproducible states, the Request is cancelled. The one exception is the
// internal periodic dirty-render tick (a nil-destination Update broadcast): the
// dirty work has already been moved into the Request's pending dirt, so the tick
// is only a nudge and can be dropped when the channel is full. The dirt is still
// rendered — a running Request is woken by the already-buffered message and drains
// it on the next pass, and one still starting up drains it on its first processing
// pass — so no work is lost. The cancellation cause reachable via [context.Cause]
// on [Request.Context] wraps this sentinel, so it can be matched with [errors.Is];
// the wrapped text identifies which channel overflowed.
var ErrRequestOverloaded = errors.New("request overloaded")

// ErrValueNotFinite indicates a [Request] was torn down because a NaN or infinite
// float64 reached the UI.
//
// A non-finite value has no valid rendering or wire representation and, in the case
// of NaN, is not even equal to itself, so it cannot be coerced safely. Rather than
// silently blanking or dropping it, the Request that produced it is cancelled. The
// cancellation cause reachable via [context.Cause] on [Request.Context] wraps this
// sentinel, so it can be matched with [errors.Is]; the wrapped text identifies the
// offending value.
var ErrValueNotFinite = errors.New("float value is not finite")

// ErrRequestAlreadyClaimed is returned when [Jaws.UseRequest] is called more than once for a [Request].
var ErrRequestAlreadyClaimed = errors.New("request already claimed")

// ErrInvalidChildElement indicates an invalid child [Element].
//
// Child operations report this error when the child is nil, deleted,
// unregistered, the receiver itself, or belongs to another [Request].
var ErrInvalidChildElement = errors.New("invalid child element")

// ErrInvalidChildIndex indicates an invalid child index.
//
// [Jaws.Insert] reports this error for a negative index. Use [Jaws.Append] to
// insert at the end.
var ErrInvalidChildIndex = errors.New("invalid child index")

// ErrReservedAttribute indicates an attempt to set or remove a framework-owned
// attribute through a public attribute helper.
//
// The "id" attribute carries an [Element]'s JaWS identity: every wire command
// resolves its target node with document.getElementById, so changing or removing
// it would strand the server-side [Element] with an unreachable DOM node. The
// [Element.SetAttr], [Element.RemoveAttr], [Jaws.SetAttr] and [Jaws.RemoveAttr]
// helpers report this via reportMisuse and send nothing.
var ErrReservedAttribute = errors.New("reserved attribute")

// ErrJavascriptDisabled is returned when the noscript probe indicates JavaScript is disabled.
var ErrJavascriptDisabled = errors.New("javascript is disabled")

// ErrWebsocketOriginMissing is returned when a WebSocket request has no Origin header.
var ErrWebsocketOriginMissing = errors.New("websocket request missing Origin header")

// ErrWebsocketOriginWrongScheme is returned when a WebSocket Origin is not HTTP or HTTPS.
var ErrWebsocketOriginWrongScheme = errors.New("websocket Origin not http or https")

// ErrWebsocketOriginWrongHost is returned when a WebSocket Origin host does not match the initial request host.
var ErrWebsocketOriginWrongHost = errors.New("websocket Origin host mismatch")

// ErrWebsocketOriginNoInitial is returned when origin validation cannot run
// because the [Request] has no initial HTTP request to compare against. The
// check fails closed rather than accepting an unverified Origin.
var ErrWebsocketOriginNoInitial = errors.New("websocket Origin cannot be validated: no initial request")

// ErrRequestCancelled indicates a [Request] was cancelled.
//
// The concrete error reachable via [context.Cause] on [Request.Context] wraps the
// underlying cancellation cause, so it can be matched with [errors.Is] and its cause
// retrieved with Unwrap. The exported sentinel itself carries no cause.
var ErrRequestCancelled errRequestCancelled

type errRequestCancelled struct {
	JawsKey    key.Key
	Cause      error
	Method     string // initial request method
	RequestURI string // initial request URI
	HasInitial bool   // whether the Request had an initial HTTP request
}

func (e errRequestCancelled) Error() string {
	if e.HasInitial {
		return fmt.Sprintf("Request<%s>: %s %q: %v", e.JawsKey, e.Method, e.RequestURI, e.Cause)
	}
	return fmt.Sprintf("Request<%s>: %v", e.JawsKey, e.Cause)
}

func (e errRequestCancelled) Is(target error) bool {
	return target == ErrRequestCancelled
}

func (e errRequestCancelled) Unwrap() error {
	return e.Cause
}

func newErrRequestCancelledLocked(rq *Request, cause error) (err error) {
	if cause != nil {
		ec := errRequestCancelled{
			JawsKey: rq.JawsKey,
			Cause:   cause,
		}
		if rq.initial != nil {
			ec.HasInitial = true
			ec.Method = rq.initial.Method
			ec.RequestURI = rq.initial.RequestURI
		}
		err = ec
	}
	return
}

// ErrWebSocketIPMismatch is returned when the WebSocket callback for a
// [Request] arrives from a different client IP than the initial HTTP request.
var ErrWebSocketIPMismatch errWebSocketIPMismatch

type errWebSocketIPMismatch struct {
	JawsKey  string
	Expected netip.Addr
	Actual   netip.Addr
}

func (e errWebSocketIPMismatch) Error() string {
	return fmt.Sprintf("/jaws/%s: expected IP %q, got %q", e.JawsKey, e.Expected.String(), e.Actual.String())
}

func (e errWebSocketIPMismatch) Is(target error) bool {
	return target == ErrWebSocketIPMismatch
}

// newErrWebSocketIPMismatchLocked reads rq fields; caller must hold rq.mu.
//
// It reads rq.JawsKey directly rather than via [Request.JawsKeyString], which
// takes rq.mu and would deadlock here since the caller already holds it.
func newErrWebSocketIPMismatchLocked(rq *Request, actual netip.Addr) error {
	return errWebSocketIPMismatch{JawsKey: rq.JawsKey.String(), Expected: rq.remoteIP, Actual: actual}
}

// ErrTooManyPendingRequests indicates an older pending Request was evicted
// because its client IP had reached [Jaws.MaxPendingRequestsPerIP].
var ErrTooManyPendingRequests errTooManyPendingRequests

type errTooManyPendingRequests struct {
	Addr  netip.Addr
	Limit int
}

func (e errTooManyPendingRequests) Error() string {
	return fmt.Sprintf("too many pending requests from %v (limit %d)", e.Addr, e.Limit)
}

func (e errTooManyPendingRequests) Is(target error) bool {
	return target == ErrTooManyPendingRequests
}

func newErrTooManyPendingRequests(remoteIP netip.Addr, limit int) error {
	return errTooManyPendingRequests{Addr: remoteIP, Limit: limit}
}

// ErrNoWebSocketRequest is returned when the WebSocket callback was not received
// within the timeout period. The most common reason is that the client is not
// using JavaScript.
var ErrNoWebSocketRequest errNoWebSocketRequest

type errNoWebSocketRequest struct {
	Addr netip.Addr
}

func (e errNoWebSocketRequest) Error() string {
	return fmt.Sprintf("no WebSocket request received from %v", e.Addr)
}

func (e errNoWebSocketRequest) Is(target error) bool {
	return target == ErrNoWebSocketRequest
}

func newErrNoWebSocketRequest(rq *Request) error {
	return errNoWebSocketRequest{Addr: rq.remoteIP}
}

// ErrEventHandlerPanic is returned by [CallEventHandlers] when a user event handler
// panics.
//
// Match it with [errors.Is]. When the recovered panic value is itself an error it is
// available via Unwrap (and thus [errors.As] / [errors.Is]); a non-error panic value
// appears only in the formatted message.
var ErrEventHandlerPanic errEventHandlerPanic

type errEventHandlerPanic struct {
	// Type is the [Element]'s UI object type. Handlers registered on the Element are
	// tried before the UI object, so the type that actually panicked may differ from
	// this when a registered handler is the culprit.
	Type  reflect.Type
	Value any // the recovered panic value
}

func (e errEventHandlerPanic) Error() string {
	return fmt.Sprintf("jaws: %v panic: %v", e.Type, e.Value)
}

func (errEventHandlerPanic) Is(target error) bool {
	return target == ErrEventHandlerPanic
}

func (e errEventHandlerPanic) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

type errEventUnhandled struct{}

func (errEventUnhandled) Error() string {
	return "event unhandled"
}

// ErrEventUnhandled returned by [InputHandler.JawsInput], [ClickHandler.JawsClick]
// or [ContextMenuHandler.JawsContextMenu] causes the next available handler to be invoked.
var ErrEventUnhandled = errEventUnhandled{}
