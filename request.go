package jaws

import (
	"cmp"
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/secureheaders"
)

// webSocketReadLimit bounds the size of a single inbound WebSocket message. It
// matches the current coder/websocket default (32 KiB); larger messages fail the
// read and close the connection. We set it explicitly so the cap is part of jaws'
// own contract and cannot change silently if the library default does.
const webSocketReadLimit = 32 * 1024

// ConnectFn can be used to interact with a [Request] before message processing starts.
// Returning an error causes the [Request] to abort, and the WebSocket connection to close.
type ConnectFn = func(rq *Request) error

type requestBuffers struct {
	todoDirt []any
	elems    []*Element
	tagMap   map[any][]*Element
	wsQueue  []wire.WsMsg
}

// Request maintains the state for a JaWS WebSocket connection, and handles processing
// of events and broadcasts.
//
// Each Request has a stable identity and is never reused for another connection.
// After it finishes, its context remains canceled and identity-targeted operations
// are not retargeted to another Request.
//
// Unlike [Session], whose methods are nil-safe, Request methods are not safe to call on a
// nil *Request: a Request is always obtained from [Jaws.NewRequest] or [Jaws.UseRequest]
// and is never legitimately nil. The nil-receiver guard on [Request.JawsKeyString] (and
// thus [Request.String]) lets a nil Request render into error text, while those on
// [Request.Log] and [Request.MustLog] let it forward to the logger; both exist only for
// that diagnostic use, not as a public nil-safe contract.
type Request struct {
	Jaws             *Jaws                   // (read-only) the JaWS instance the Request belongs to
	JawsKey          key.Key                 // (read-only) random key assigned to this Request; routes JaWS URLs and request-targeted broadcasts only while registered
	remoteIP         netip.Addr              // (read-only) remote IP, or the zero netip.Addr if unset
	registered       bool                    // present as a live identity in Jaws.requests; guarded by mu
	running          atomic.Bool             // if ServeHTTP() is running
	claimed          atomic.Bool             // if UseRequest() has been called for it
	lastWriteSeconds atomic.Int32            // [Jaws.runtimeSeconds] value at the most recent RequestWriter write; lock-free, drives pending-eviction recency (oldestEvictablePendingLocked) and idle expiry (maintenance)
	mu               deadlock.RWMutex        // protects following
	lastJid          Jid                     // last element Jid allocated within this Request
	initial          *http.Request           // initial HTTP request passed to Jaws.NewRequest
	session          *Session                // session, if established
	todoDirt         []any                   // dirty tags
	ctx              context.Context         // current context, derived from either Jaws or WS HTTP req; stored in the struct because there is no call chain between Request creation and its use once the WebSocket exists
	httpDoneCh       <-chan struct{}         // once claimed, set to http.Request.Context().Done()
	cancelFn         context.CancelCauseFunc // cancel function
	connectFn        ConnectFn               // a ConnectFn to call before starting message processing for the Request
	buffers          *requestBuffers         // detached and reused after this Request finishes
	elems            []*Element              // our Elements
	tagMap           map[any][]*Element      // maps tags to Elements
	muQueue          deadlock.Mutex          // protects wsQueue and tailsent
	wsQueue          []wire.WsMsg            // queued messages to send
	tailsent         bool
}

type eventFnCall struct {
	jid  Jid
	wht  what.What
	data string
}

// JawsKeyString returns the request key in the text form used by JaWS URLs.
func (rq *Request) JawsKeyString() string {
	jawsKey := key.Key(0)
	if rq != nil {
		rq.mu.RLock()
		jawsKey = rq.JawsKey
		rq.mu.RUnlock()
	}
	return jawsKey.String()
}

// String returns the Request in the form "Request<key>", using [Request.JawsKeyString]
// to encode the key. Like JawsKeyString it tolerates a nil receiver for diagnostics
// only; see the [Request] type documentation.
func (rq *Request) String() string {
	return "Request<" + rq.JawsKeyString() + ">"
}

// MarkWritten records that the Request's initial HTML is being written, so the
// pending-eviction logic spares it while a render is in flight.
//
// [RequestWriter.Write] calls it on every write. It is lock-free and safe to call
// concurrently.
func (rq *Request) MarkWritten() {
	// A single atomic store of the cached runtimeSeconds; no clock read. The recorded
	// second drives the recency window in oldestEvictablePendingLocked and the idle
	// expiry in maintenance.
	rq.lastWriteSeconds.Store(rq.Jaws.runtimeSeconds.Load())
}

// destKey returns the Request's current identity key, read under rq.mu, for use as
// a broadcast destination. Targeting the key value rather than the *Request pointer
// lets the Serve loop reject messages after the Request is unregistered. A zero
// return means the Request is no longer a valid target.
func (rq *Request) destKey() (k key.Key) {
	rq.mu.RLock()
	if rq.registered {
		k = rq.JawsKey
	}
	rq.mu.RUnlock()
	return
}

// claim binds this request to the HTTP request making the WebSocket call. It
// verifies the client IP matches, then atomically marks the request claimed and
// layers a fresh cancelable context over the current one (preserving any context
// installed via SetContext, whose cancelFn is still chained so it runs on
// cleanup). Returns [ErrWebSocketIPMismatch] if the client IP does not match,
// or [ErrRequestAlreadyClaimed] if it was already claimed.
func (rq *Request) claim(r *http.Request) error {
	if !rq.claimed.Load() {
		var actualIP netip.Addr
		var httpDoneCh <-chan struct{}
		if r != nil { // can be nil in tests
			actualIP = rq.Jaws.clientIP(r)
			httpDoneCh = r.Context().Done()
		}
		rq.mu.Lock()
		defer rq.mu.Unlock()
		if !equalIP(rq.remoteIP, actualIP) {
			return newErrWebSocketIPMismatchLocked(rq, actualIP)
		}
		if rq.ctx.Err() != nil {
			return context.Cause(rq.ctx)
		}
		if rq.claimed.CompareAndSwap(false, true) {
			// Layer a fresh cancelable context over the current one (which may
			// have been customized via SetContext) so the claim has its own
			// cancel handle. The previous cancelFn must still be invoked on
			// cleanup, otherwise the context it created leaks in the parent
			// (typically Jaws.BaseContext) until that parent is cancelled.
			prevCancel := rq.cancelFn
			rq.ctx, rq.cancelFn = context.WithCancelCause(rq.ctx)
			if prevCancel != nil {
				newCancel := rq.cancelFn
				rq.cancelFn = func(cause error) {
					newCancel(cause)
					prevCancel(cause)
				}
			}
			rq.httpDoneCh = httpDoneCh
			// Refresh the write second so a request claimed long after its initial
			// render (a throttled or backgrounded tab) is not treated as idle and
			// retired in the window before ServeHTTP sets running.
			rq.lastWriteSeconds.Store(rq.Jaws.runtimeSeconds.Load())
			return nil
		}
	}
	return ErrRequestAlreadyClaimed
}

func (rq *Request) killSessionLocked() {
	if rq.session != nil {
		rq.session.delRequest(rq)
		rq.session = nil
	}
}

func (rq *Request) killSession() {
	rq.mu.Lock()
	rq.killSessionLocked()
	rq.mu.Unlock()
}

// deadSession detaches sess and returns the Request identity that belonged to it.
// A zero return means rq has finished or belongs to another Session.
func (rq *Request) deadSession(sess *Session) (k key.Key) {
	rq.mu.Lock()
	if rq.session == sess {
		rq.session = nil
		k = rq.JawsKey
	}
	rq.mu.Unlock()
	return
}

// sessionDestKey returns rq's identity only while it still belongs to sess.
func (rq *Request) sessionDestKey(sess *Session) (k key.Key) {
	rq.mu.RLock()
	if rq.session == sess {
		k = rq.JawsKey
	}
	rq.mu.RUnlock()
	return
}

func (rq *Request) ensureAutoSession(w http.ResponseWriter, r *http.Request) {
	if rq.Jaws.AutoSession && rq.Session() == nil {
		sess := rq.Jaws.newSession(w, r)
		rq.mu.Lock()
		if rq.session == nil {
			rq.session = sess
			sess.addRequest(rq)
		}
		rq.mu.Unlock()
	}
}

// releaseBuffersLocked detaches reusable storage from a finished Request.
//
// It releases queued dirt and messages, marks elements deleted, and detaches the
// Request from its session. The Request keeps its identity and canceled context,
// so retained pointers remain permanently associated with this lifecycle. The
// caller must hold rq.mu and ensure request processing has stopped.
func (rq *Request) releaseBuffersLocked() (buffers *requestBuffers) {
	if rq.cancelFn != nil {
		rq.cancelFn(nil)
	}
	rq.lastJid = 0
	rq.connectFn = nil
	rq.initial = nil
	rq.killSessionLocked()
	rq.running.Store(false)
	rq.claimed.Store(false)
	rq.registered = false
	rq.httpDoneCh = nil
	clear(rq.todoDirt)
	rq.todoDirt = rq.todoDirt[:0]
	rq.remoteIP = netip.Addr{}
	for _, e := range rq.elems {
		if e != nil {
			// Nil the GC-reachable fields and set the deleted guard, which makes any
			// retained *Element inert (see [Element.Deleted]). Elements are allocated
			// fresh per newElementLocked and are never pooled.
			e.handlers = nil
			e.ui = nil
			e.deleted.Store(true)
		}
	}
	clear(rq.elems)
	rq.elems = rq.elems[:0]
	// wsQueue and tailsent are guarded by muQueue, not rq.mu. A /jaws/.tail/<key>
	// fetch (drainTailScript) on a still-pending request runs on a separate HTTP
	// goroutine holding only muQueue, so take muQueue here to serialize the reset
	// with it; the documented rq.mu -> muQueue lock order is preserved since we
	// already hold rq.mu.
	rq.muQueue.Lock()
	rq.tailsent = false
	clear(rq.wsQueue)
	rq.wsQueue = rq.wsQueue[:0]
	rq.muQueue.Unlock()
	clear(rq.tagMap)

	buffers = rq.buffers
	if buffers != nil {
		buffers.todoDirt = rq.todoDirt
		buffers.elems = rq.elems
		buffers.tagMap = rq.tagMap
		buffers.wsQueue = rq.wsQueue
	}
	rq.buffers = nil
	rq.todoDirt = nil
	rq.elems = nil
	rq.tagMap = nil
	rq.wsQueue = nil
	return
}

// HeadHTML writes the configured resources and Request key metadata for the page head.
func (rq *Request) HeadHTML(w io.Writer) (err error) {
	rq.mu.RLock()
	jawsKey := rq.JawsKey
	rq.mu.RUnlock()
	var b []byte
	rq.Jaws.mu.RLock()
	b = append(b, rq.Jaws.headPrefix...)
	rq.Jaws.mu.RUnlock()
	b = key.Append(b, jawsKey)
	b = append(b, `">`...)
	_, err = w.Write(b)
	return
}

// GetConnectFn returns the currently set [ConnectFn].
// That function will be called before starting the WebSocket tunnel if not nil.
func (rq *Request) GetConnectFn() (fn ConnectFn) {
	rq.mu.RLock()
	fn = rq.connectFn
	rq.mu.RUnlock()
	return
}

// SetConnectFn sets the [ConnectFn].
// That function will be called before starting the WebSocket tunnel if not nil.
func (rq *Request) SetConnectFn(fn ConnectFn) {
	rq.mu.Lock()
	rq.connectFn = fn
	rq.mu.Unlock()
}

// Session returns the Request's Session, or nil.
func (rq *Request) Session() (sess *Session) {
	rq.mu.RLock()
	sess = rq.session
	rq.mu.RUnlock()
	return
}

// Initial returns the Request's initial HTTP request, or nil.
func (rq *Request) Initial() (r *http.Request) {
	rq.mu.RLock()
	r = rq.initial
	rq.mu.RUnlock()
	return
}

// Get is shorthand for [Session.Get].
//
// It returns the session value associated with key, or nil if no session is associated
// with the [Request].
func (rq *Request) Get(key string) any {
	return rq.Session().Get(key)
}

// Set is shorthand for [Session.Set].
//
// It associates value with key in the session; a nil value removes the key. It does
// nothing if no session is associated with the [Request].
func (rq *Request) Set(key string, value any) {
	rq.Session().Set(key, value)
}

// Context returns the [Request]'s context, which is by default derived from [Jaws.BaseContext].
func (rq *Request) Context() (ctx context.Context) {
	rq.mu.RLock()
	ctx = rq.ctx
	rq.mu.RUnlock()
	return
}

// SetContext atomically transforms the Request's context.
//
// fn receives the current context and must return a non-nil context derived from
// it so cancellation and deadlines continue to propagate. Cancellation or
// deadline expiration of the returned context wakes a running
// [Request.ServeHTTP] loop promptly, even while it is idle; no WebSocket event or
// broadcast is required.
//
// fn runs while the Request lock is held. It must not call methods on the same
// Request, call code that may do so, or block on work that needs the same
// Request. SetContext panics if fn is nil. If fn panics, SetContext releases the
// lock and propagates the panic.
//
// Returning a nil context is a programming error: debug builds panic and production
// builds report it through [Jaws.MustLog] and retain the current context.
func (rq *Request) SetContext(fn func(oldCtx context.Context) (newCtx context.Context)) {
	oldCtx, newCtx, cancelFn := rq.replaceContext(fn)
	if newCtx == nil {
		rq.Jaws.reportMisuse(errors.New("jaws: SetContext function returned a nil context"))
		return
	}
	if newCtx.Done() == oldCtx.Done() {
		// The request loop already observes this Done channel, so no cancellation
		// bridge is needed.
		return
	}
	// The request loop may already be blocked selecting on oldCtx.Done. Bridge a
	// replacement's cancellation into the allocation's stable cancel function so
	// that old select wakes. Register outside rq.mu because context.AfterFunc may
	// synchronously invoke a custom context hook that re-enters the Request.
	//
	// Capture only the context and cancel closure so the callback does not retain
	// the entire Request and its rendered state.
	context.AfterFunc(newCtx, func() {
		cancelFn(context.Cause(newCtx))
	})
}

func (rq *Request) replaceContext(fn func(oldCtx context.Context) (newCtx context.Context)) (oldCtx, newCtx context.Context, cancelFn context.CancelCauseFunc) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	oldCtx = rq.ctx
	if newCtx = fn(oldCtx); newCtx != nil {
		rq.ctx = newCtx
		cancelFn = rq.cancelFn
	}
	return
}

// maintenance reports whether rq has expired and should be retired. For a
// request that never went live it cancels and reports expiry once it has been
// idle (no [RequestWriter] write) longer than requestTimeout, or immediately if its
// context is already done. nowSeconds is the reference instant ([Jaws.runtimeSeconds]).
// Called from the Serve loop's maintenance pass while jw.mu is held.
//
// It returns the cancellation cause (or nil) rather than logging it, so the caller
// can log it after releasing jw.mu — logging runs the user [Jaws.Logger], which
// must not be invoked under a lock.
func (rq *Request) maintenance(nowSeconds int32, requestTimeout time.Duration) (expired bool, cause error) {
	if !rq.running.Load() {
		rq.mu.Lock()
		if rq.ctx.Err() != nil {
			expired = true
		} else {
			elapsedSeconds := nowSeconds - rq.lastWriteSeconds.Load()
			if elapsedSeconds > 0 && time.Duration(elapsedSeconds)*time.Second > requestTimeout {
				cause = rq.cancelLocked(newErrNoWebSocketRequest(rq))
				expired = true
			}
		}
		rq.mu.Unlock()
	}
	return
}

// cancelLocked cancels the Request's context, wrapping a non-nil cause, when the
// Request has a non-zero identity key and has not already been canceled.
//
// It does NOT log. It returns the cancellation cause (already set on the context)
// so the caller can pass it to [Jaws.Log] AFTER releasing rq.mu and any outer lock;
// the cause is nil whenever the context was already canceled or err is nil.
// Logging invokes the user-supplied [Jaws.Logger], which the package locking
// contract forbids running under a lock. Caller must hold rq.mu.
func (rq *Request) cancelLocked(err error) (cause error) {
	if rq.JawsKey != 0 && rq.ctx.Err() == nil {
		cause = newErrRequestCancelledLocked(rq, err)
		rq.cancelFn(cause)
	}
	return
}

// cancel locks rq.mu, cancels the context, then logs the cancellation cause after
// releasing the lock ([Jaws.Log] is a no-op on a nil cause).
func (rq *Request) cancel(err error) {
	rq.mu.Lock()
	cause := rq.cancelLocked(err)
	rq.mu.Unlock()
	_ = rq.Jaws.Log(cause)
}

// Cancel aborts the Request.
//
// It cancels the Request's context with the given cause (logged via [Jaws.Logger]);
// the WebSocket processing loop and its goroutines observe the cancelled context and
// shut down asynchronously. Cancel returns immediately and does not wait for teardown.
// It is safe to call from UI code, for example to terminate a connection that violates
// a server-side limit. A nil err cancels without a specific cause, and calling Cancel
// on an already-finished or already-cancelled Request has no effect.
func (rq *Request) Cancel(err error) {
	rq.cancel(err)
}

// alertData builds the wire payload for an Alert message, HTML-escaping both the
// level and the message. Callers may therefore pass untrusted text safely; this
// mirrors the escaping done by the internal [wire.WsMsg.FillAlert] path.
func alertData(level, msg string) string {
	return html.EscapeString(level) + "\n" + html.EscapeString(msg)
}

// Alert attempts to show an alert message on the current request webpage if it
// has an HTML element with the data-jaws-alerts attribute.
//
// The level argument should be one of Bootstrap's alert levels: primary, secondary, success, danger, warning, info, light or dark.
//
// The level and msg are HTML-escaped before being sent, so it is safe to pass
// untrusted text; do not pre-escape it.
//
// The default JaWS JavaScript only supports Bootstrap dismissible alerts.
//
// It does nothing if the Request has already finished, since it then has no
// live target. See [Jaws.Broadcast] for processing-loop requirements.
func (rq *Request) Alert(level, msg string) {
	if k := rq.destKey(); k != 0 {
		rq.Jaws.Broadcast(wire.Message{
			Dest: k,
			What: what.Alert,
			Data: alertData(level, msg),
		})
	}
}

// AlertError logs err via [Jaws.Log] and, if it is non-nil, also shows it to the
// current request as a danger-level [Request.Alert].
func (rq *Request) AlertError(err error) {
	if rq.Jaws.Log(err) != nil {
		rq.Alert("danger", err.Error())
	}
}

// Redirect requests the current [Request] to navigate to the given URL.
//
// The URL is validated to be a relative path or an http/https URL; script-bearing
// schemes such as javascript: and protocol-relative ("//host") URLs are refused
// and logged rather than sent to the browser.
//
// It does nothing if the Request has already finished, since it then has no
// live target. See [Jaws.Broadcast] for processing-loop requirements.
func (rq *Request) Redirect(url string) {
	if msg, ok := rq.Jaws.redirectMessage(url); ok {
		if k := rq.destKey(); k != 0 {
			msg.Dest = k
			rq.Jaws.Broadcast(msg)
		}
	}
}

// tagsOfLocked returns the tags currently associated with elem. Caller must hold
// rq.mu (read or write).
func (rq *Request) tagsOfLocked(elem *Element) (tags []any) {
	for tagValue, elems := range rq.tagMap {
		if slices.Contains(elems, elem) {
			tags = append(tags, tagValue)
		}
	}
	return
}

// TagsOf returns the tags currently associated with elem in this Request, or nil
// if elem is nil. The returned slice is a newly allocated snapshot and may be
// retained and modified by the caller.
func (rq *Request) TagsOf(elem *Element) (tags []any) {
	if elem != nil {
		rq.mu.RLock()
		defer rq.mu.RUnlock()
		tags = rq.tagsOfLocked(elem)
	}
	return
}

// Dirty marks all [Element] values that have one or more of the given tags as dirty.
func (rq *Request) Dirty(dirtyTags ...any) {
	rq.Jaws.setDirty(tag.MustTagExpand(rq, dirtyTags))
}

// wantMessage returns true if the Request want the message.
func (rq *Request) wantMessage(msg *wire.Message) (yes bool) {
	switch dest := msg.Dest.(type) {
	case key.Key: // the request with this identity key
		rq.mu.RLock()
		if rq.registered {
			yes = rq.JawsKey == dest
		}
		rq.mu.RUnlock()
		return
	case []any: // more than one tag
		rq.mu.RLock()
		defer rq.mu.RUnlock()
		for i := range dest {
			if _, yes = rq.tagMap[dest[i]]; yes {
				break
			}
		}
	default:
		rq.mu.RLock()
		defer rq.mu.RUnlock()
		_, yes = rq.tagMap[msg.Dest]
	}
	return
}

// newElementLocked allocates an [Element] wrapping ui, assigning it the next Jid
// and appending it to the request's element list. Caller must hold rq.mu.
func (rq *Request) newElementLocked(ui UI) (elem *Element) {
	rq.lastJid++
	elem = &Element{
		jid:     rq.lastJid,
		ui:      ui,
		Request: rq,
	}
	rq.elems = append(rq.elems, elem)
	return
}

// NewElement creates a new [Element] using the given [UI] object.
//
// Panics if the build tag "debug" is set and the [UI] object doesn't satisfy all requirements.
func (rq *Request) NewElement(ui UI) *Element {
	if deadlock.Debug {
		if err := tag.NewErrNotComparable(ui); err != nil {
			panic(err)
		}
	}
	rq.mu.Lock()
	defer rq.mu.Unlock()
	return rq.newElementLocked(ui)
}

// GetElementByJid returns the element with jid, or nil if it is not known.
func (rq *Request) GetElementByJid(jid Jid) (elem *Element) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	elem = rq.getElementByJidLocked(jid)
	return
}

// getElementByJidLocked returns the element with jid, or nil. Caller must hold
// rq.mu (read or write).
func (rq *Request) getElementByJidLocked(jid Jid) (elem *Element) {
	// rq.elems is kept sorted ascending by Jid (newElementLocked appends after
	// incrementing lastJid; deletes preserve order), so binary search resolves a
	// Jid in O(log n). Jids are not dense (deletes leave gaps) so we cannot index
	// rq.elems by Jid directly.
	if deadlock.Debug && !slices.IsSortedFunc(rq.elems, func(a, b *Element) int {
		return cmp.Compare(a.Jid(), b.Jid())
	}) {
		// A future insertion path that breaks the ordering would make the binary
		// search below silently miss elements; fail loudly in debug builds (CI runs
		// -tags debug -race) instead of returning wrong lookups in production.
		panic("jaws: rq.elems not sorted ascending by Jid")
	}
	if i, ok := slices.BinarySearchFunc(rq.elems, jid, func(e *Element, target Jid) int {
		return cmp.Compare(e.Jid(), target)
	}); ok {
		elem = rq.elems[i]
	}
	return
}

// hasTagLocked reports whether elem is registered under tagValue. Caller must
// hold rq.mu (read or write).
func (rq *Request) hasTagLocked(elem *Element, tagValue any) bool {
	return slices.Contains(rq.tagMap[tagValue], elem)
}

// HasTag reports whether elem has tagValue in rq.
func (rq *Request) HasTag(elem *Element, tagValue any) (yes bool) {
	rq.mu.RLock()
	yes = rq.hasTagLocked(elem, tagValue)
	rq.mu.RUnlock()
	return
}

// appendDirtyTags queues already-expanded tags onto this request's pending-dirt
// list. The Serve loop's update tick later drains the list (see makeUpdateList)
// and re-renders the affected elements. Takes rq.mu.
//
// It may run after the caller's dirt snapshot was unregistered. In that case the
// tags are discarded rather than retained on the finished Request.
func (rq *Request) appendDirtyTags(tags []any) {
	rq.mu.Lock()
	if rq.registered {
		rq.todoDirt = append(rq.todoDirt, tags...)
	}
	rq.mu.Unlock()
}

// TagExpanded adds already-expanded tags to the given [Element].
func (rq *Request) TagExpanded(elem *Element, expandedTags []any) {
	if elem != nil && !elem.deleted.Load() && elem.Request == rq {
		rq.mu.Lock()
		defer rq.mu.Unlock()
		for _, tagValue := range expandedTags {
			if !rq.hasTagLocked(elem, tagValue) {
				rq.tagMap[tagValue] = append(rq.tagMap[tagValue], elem)
			}
		}
	}
}

// Tag adds the given tags to the given [Element].
func (rq *Request) Tag(elem *Element, tagItems ...any) {
	if elem != nil && len(tagItems) > 0 && elem.Request == rq {
		rq.TagExpanded(elem, tag.MustTagExpand(elem.Request, tagItems))
	}
}

// GetElements returns a list of the UI elements in the [Request] that have the given tags.
func (rq *Request) GetElements(tagValue any) (elems []*Element) {
	expanded := tag.MustTagExpand(rq, tagValue)
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	if len(expanded) == 1 {
		// The common single-tag case needs no de-duplication: rq.tagMap[tag] is
		// already duplicate-free. Clone it (callers like handleBroadcast mutate
		// tagMap after the lock is released, so we must not alias it).
		return slices.Clone(rq.tagMap[expanded[0]])
	}
	seen := map[*Element]struct{}{}
	for _, tagValue := range expanded {
		if el, ok := rq.tagMap[tagValue]; ok {
			for _, e := range el {
				if _, ok = seen[e]; !ok {
					seen[e] = struct{}{}
					elems = append(elems, e)
				}
			}
		}
	}
	return
}

// validateWebSocketOrigin checks the WebSocket upgrade's Origin header against the
// page that served the initial request, defending against cross-origin WebSocket
// hijacking. It requires the Origin to be present, to use a scheme matching the
// initial request's security (http when plain, https when secure), and to have a
// host equal to the initial host (case-insensitive, default port stripped). It
// returns a specific ErrWebsocketOrigin* error on each failure mode and nil only
// on a full match. If there is no initial request to compare against, it fails
// closed with ErrWebsocketOriginNoInitial rather than accepting the Origin.
func (rq *Request) validateWebSocketOrigin(r *http.Request) (err error) {
	err = ErrWebsocketOriginMissing
	if origin := r.Header.Get("Origin"); origin != "" {
		var u *url.URL
		if u, err = url.Parse(origin); err == nil {
			// Fail closed if the parse succeeded but there is nothing to compare
			// the Origin against; otherwise the nil err from url.Parse would
			// silently accept any Origin.
			err = ErrWebsocketOriginNoInitial
			if initial := rq.Initial(); initial != nil {
				secure := secureheaders.RequestIsSecure(initial, rq.Jaws.TrustForwardedHeaders)
				port := ""
				uhost := u.Host
				ihost := initial.Host
				err = ErrWebsocketOriginWrongScheme
				switch u.Scheme {
				case "http":
					if secure {
						return
					}
					port = ":80"
				case "https":
					if !secure {
						return
					}
					port = ":443"
				default:
					return
				}
				uhost = strings.TrimSuffix(uhost, port)
				ihost = strings.TrimSuffix(ihost, port)
				err = ErrWebsocketOriginWrongHost
				if uhost != "" {
					// Browser WebSocket requests use the page origin.
					// Compare both scheme and host against the initial request.
					if strings.EqualFold(uhost, ihost) {
						err = nil
					}
				}
			}
		}
	}
	return
}

// Log sends an error to the [Jaws.Logger] if set.
// Has no effect if err is nil or the Logger is nil.
// Returns err.
func (rq *Request) Log(err error) error {
	var jw *Jaws
	if rq != nil {
		jw = rq.Jaws
	}
	return jw.Log(err)
}

// MustLog sends an error to the [Jaws.Logger] if set, or
// panics with the given error if the Logger is nil.
// Has no effect if err is nil.
//
// Some update-time paths cannot return errors to their caller and report them
// through MustLog. Set [Jaws.Logger] when those errors should be logged instead
// of treated as fatal programming errors.
func (rq *Request) MustLog(err error) {
	var jw *Jaws
	if rq != nil {
		jw = rq.Jaws
	}
	jw.MustLog(err)
}

// startServe transitions a claimed request to running.
//
// It takes jw.mu so the running transition is serialized with the maintenance
// pass, which also holds jw.mu when deciding whether to retire a not-running
// request. The map identity check keeps a retired Request from starting even
// though its fields remain intact for an initial HTTP handler that still owns it.
func (rq *Request) startServe() (ok bool) {
	rq.Jaws.mu.Lock()
	defer rq.Jaws.mu.Unlock()
	select {
	case <-rq.Jaws.closeCh:
		return false
	default:
	}
	rq.mu.RLock()
	registered := rq.Jaws.requests[rq.JawsKey] == rq
	contextLive := rq.ctx != nil && rq.ctx.Err() == nil
	rq.mu.RUnlock()
	return registered && contextLive && rq.claimed.Load() && rq.running.CompareAndSwap(false, true)
}

func (rq *Request) stopServe() {
	rq.cancel(nil)
	rq.Jaws.recycle(rq)
}

// ServeHTTP implements [http.Handler].
//
// Requires [Jaws.UseRequest] to have been successfully called for the [Request].
// The JaWS processing loop ([Jaws.Serve] or [Jaws.ServeWithTimeout]) must also
// be running so the request can subscribe to broadcasts and unsubscribe on exit.
func (rq *Request) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rq.startServe() {
		defer rq.stopServe()
		pingInterval := rq.Jaws.WebSocketPingInterval
		wsTimeout := rq.Jaws.getWebSocketTimeout()
		if strings.HasSuffix(r.URL.Path, "/noscript") {
			w.WriteHeader(http.StatusNoContent)
			rq.cancel(ErrJavascriptDisabled)
			return
		}
		var err error
		if r.Header.Get("Sec-WebSocket-Key") != "" {
			if err = rq.validateWebSocketOrigin(r); err != nil {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				rq.cancel(err)
				return
			}
			rq.ensureAutoSession(w, r)
		}
		var ws *websocket.Conn
		ws, err = websocket.Accept(w, r, nil)
		if err == nil {
			ws.SetReadLimit(webSocketReadLimit)
			if err = rq.onConnect(); err == nil {
				incomingMsgCh := make(chan wire.WsMsg)
				// Snapshot ctx, cancelFn and the element count together under the
				// lock; every other access to rq.elems is also guarded by rq.mu.
				rq.mu.RLock()
				ctx := rq.ctx
				cancelFn := rq.cancelFn
				numElems := len(rq.elems)
				rq.mu.RUnlock()
				// Size the broadcast buffer with headroom that scales with the
				// page's element count. mustBroadcast (see Jaws.Serve) sends here
				// non-blocking and, for any non-Update message, kills the
				// subscription and cancels this request if the send would block.
				// A larger page can be the target of more concurrent broadcasts
				// between drains, so the buffer grows per element (4) over a small
				// fixed base (4) to avoid spuriously cancelling a slow request.
				broadcastMsgCh := rq.Jaws.subscribe(rq, 4+numElems*4)
				outboundMsgCh := make(chan wire.WsMsg, cap(broadcastMsgCh))
				go wire.ReadLoop(ctx, cancelFn, rq.Jaws.Done(), incomingMsgCh, ws)  // closes incomingMsgCh
				go wire.WriteLoop(ctx, cancelFn, rq.Jaws.Done(), outboundMsgCh, ws) // calls ws.Close()
				go wire.PingLoop(ctx, cancelFn, rq.Jaws.Done(), pingInterval, wsTimeout, ws)
				rq.process(broadcastMsgCh, incomingMsgCh, outboundMsgCh) // unsubscribes broadcastMsgCh, closes outboundMsgCh
			} else {
				reason := err.Error()
				defer func() { _ = ws.Close(websocket.StatusNormalClosure, reason) }()
				var msg wire.WsMsg
				msg.FillAlert(rq.Jaws.Log(err))
				// Best-effort alert on a connection we're about to close; the
				// underlying error was already logged above via rq.Jaws.Log.
				_ = ws.Write(r.Context(), websocket.MessageText, msg.Append(nil))
			}
		}
		rq.cancel(err)
	} else {
		// The Request was never claimed (UseRequest not called) or is already
		// being served; either way its single-use token is invalid, so
		// surface an explicit error rather than returning an empty 200 OK.
		http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
	}
}
