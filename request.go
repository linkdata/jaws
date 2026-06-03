package jaws

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/jid"
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

// Request maintains the state for a JaWS WebSocket connection, and handles processing
// of events and broadcasts.
//
// Note that we have to store the context inside the struct because there is no call chain
// between the Request being created and it being used once the WebSocket is created.
type Request struct {
	Jaws       *Jaws                   // (read-only) the JaWS instance the Request belongs to
	JawsKey    uint64                  // (read-only) a random number used in the WebSocket URI to identify this Request
	remoteIP   netip.Addr              // (read-only) remote IP, or the zero netip.Addr if unset
	Rendering  atomic.Bool             // set to true by RequestWriter.Write()
	running    atomic.Bool             // if ServeHTTP() is running
	claimed    atomic.Bool             // if UseRequest() has been called for it
	mu         deadlock.RWMutex        // protects following
	lastJid    Jid                     // last element Jid allocated within this Request
	lastWrite  time.Time               // when the initial HTML was last written to, used for automatic cleanup
	initial    *http.Request           // initial HTTP request passed to Jaws.NewRequest
	session    *Session                // session, if established
	todoDirt   []any                   // dirty tags
	ctx        context.Context         // current context, derived from either Jaws or WS HTTP req
	httpDoneCh <-chan struct{}         // once claimed, set to http.Request.Context().Done()
	cancelFn   context.CancelCauseFunc // cancel function
	connectFn  ConnectFn               // a ConnectFn to call before starting message processing for the Request
	elems      []*Element              // our Elements
	tagMap     map[any][]*Element      // maps tags to Elements
	muQueue    deadlock.Mutex          // protects wsQueue and tailsent
	wsQueue    []wire.WsMsg            // queued messages to send
	tailsent   bool
}

type eventFnCall struct {
	jid  Jid
	wht  what.What
	data string
}

var (
	// ErrWebsocketOriginMissing is returned when a WebSocket request has no Origin header.
	ErrWebsocketOriginMissing = errors.New("websocket request missing Origin header")

	// ErrWebsocketOriginWrongScheme is returned when a WebSocket Origin is not HTTP or HTTPS.
	ErrWebsocketOriginWrongScheme = errors.New("websocket Origin not http or https")

	// ErrWebsocketOriginWrongHost is returned when a WebSocket Origin host does not match the initial request host.
	ErrWebsocketOriginWrongHost = errors.New("websocket Origin host mismatch")

	// ErrWebsocketOriginNoInitial is returned when origin validation cannot run
	// because the [Request] has no initial HTTP request to compare against. The
	// check fails closed rather than accepting an unverified Origin.
	ErrWebsocketOriginNoInitial = errors.New("websocket Origin cannot be validated: no initial request")

	// ErrRequestAlreadyClaimed is returned when [Jaws.UseRequest] is called more than once for a [Request].
	ErrRequestAlreadyClaimed = errors.New("request already claimed")

	// ErrJavascriptDisabled is returned when the noscript probe indicates JavaScript is disabled.
	ErrJavascriptDisabled = errors.New("javascript is disabled")
)

// JawsKeyString returns the request key in the text form used by JaWS URLs.
func (rq *Request) JawsKeyString() string {
	jawsKey := uint64(0)
	if rq != nil {
		jawsKey = rq.JawsKey
	}
	return assets.JawsKeyString(jawsKey)
}

func (rq *Request) String() string {
	return "Request<" + rq.JawsKeyString() + ">"
}

// claim binds this request to the HTTP request making the WebSocket call. It
// verifies the client IP matches, then atomically marks the request claimed and
// layers a fresh cancelable context over the current one (preserving any context
// installed via SetContext, whose cancelFn is still chained so it runs on
// cleanup). Returns ErrRequestAlreadyClaimed if it was already claimed.
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
			return fmt.Errorf("/jaws/%s: expected IP %q, got %q", rq.JawsKeyString(), rq.remoteIP.String(), actualIP.String())
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
			// Refresh lastWrite so a request claimed long after its initial render
			// (a throttled or backgrounded tab) is not treated as idle and recycled
			// in the window before ServeHTTP sets running.
			rq.lastWrite = time.Now()
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

func (rq *Request) deadSession(sess *Session) {
	rq.mu.Lock()
	if rq.session == sess {
		rq.session = nil
	}
	rq.mu.Unlock()
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

// clearLocked resets every field of rq to its zero state so it can be reused
// from the Jaws request pool: it cancels any live context, drops queued dirt and
// messages, detaches all elements and tags, and kills any attached session. It
// runs when rq is freshly allocated or being recycled, so the caller must ensure
// no other goroutine is using rq.
func (rq *Request) clearLocked() *Request {
	rq.JawsKey = 0
	rq.lastJid = 0
	rq.connectFn = nil
	rq.lastWrite = time.Time{}
	rq.initial = nil
	rq.running.Store(false)
	rq.claimed.Store(false)
	rq.Rendering.Store(false)
	if rq.cancelFn != nil {
		rq.cancelFn(nil)
	}
	rq.httpDoneCh = nil
	clear(rq.todoDirt) // release tag references before pooling; mirrors makeUpdateList
	rq.todoDirt = rq.todoDirt[:0]
	rq.remoteIP = netip.Addr{}
	for _, e := range rq.elems {
		if e != nil {
			e.handlers = nil
			e.ui = nil
		}
	}
	clear(rq.elems)
	rq.elems = rq.elems[:0]
	// wsQueue and tailsent are guarded by muQueue, not rq.mu. A /jaws/.tail/<key>
	// fetch (writeTailScript) on a still-pending request runs on a separate HTTP
	// goroutine holding only muQueue, so take muQueue here to serialize the reset
	// with it; the documented rq.mu -> muQueue lock order is preserved since we
	// already hold rq.mu.
	rq.muQueue.Lock()
	rq.tailsent = false
	rq.wsQueue = rq.wsQueue[:0]
	rq.muQueue.Unlock()
	clear(rq.tagMap)
	rq.killSessionLocked()
	return rq
}

// HeadHTML writes the HTML code needed in the HTML page's HEAD section.
func (rq *Request) HeadHTML(w io.Writer) (err error) {
	var b []byte
	rq.Jaws.mu.RLock()
	b = append(b, rq.Jaws.headPrefix...)
	rq.Jaws.mu.RUnlock()
	b = assets.JawsKeyAppend(b, rq.JawsKey)
	b = append(b, `">`...)
	_, err = w.Write(b)
	return
}

// appendJSQuote is like strconv.AppendQuote but also escapes '<' as '\x3c'
// to prevent '</script>' from closing the script block when embedded in HTML.
func appendJSQuote(b []byte, s string) []byte {
	start := len(b)
	b = strconv.AppendQuote(b, s)
	// strconv.AppendQuote never emits '<' as part of an escape sequence, so every
	// '<' in the appended region came from s. Most attribute/class fragments
	// contain none, so the common path appends straight into b with no copy.
	if bytes.IndexByte(b[start:], '<') < 0 {
		return b
	}
	rest := bytes.ReplaceAll(b[start:], []byte("<"), []byte(`\x3c`))
	return append(b[:start], rest...)
}

func (rq *Request) writeTailScript(w io.Writer) (sent bool, err error) {
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	// We deliberately do not guard against the pooled Request being recycled and
	// reused for a different connection between the /jaws/.tail lookup and this
	// drain. Triggering that would require a pending Request recycled inside the
	// microsecond window of its own in-flight tail fetch, this goroutine stalled
	// across a full recycle+reuse, and the pool returning that same object to a new
	// render that re-queued tail messages. The worst case is cosmetic and
	// self-healing — the stale tab briefly applies another request's attribute/class
	// updates to same-numbered jids, and the reused tab loses its initial
	// flicker-prevention — with no server-side state or security impact, since jids
	// are per-Request, the client already controls its own DOM, and the WebSocket
	// re-applies the authoritative state on connect. (The data race on
	// wsQueue/tailsent itself is still prevented: clearLocked takes muQueue to reset
	// them.)
	if !rq.tailsent {
		rq.tailsent = true
		sent = true
		var b []byte
		n := 0
		for _, msg := range rq.wsQueue {
			var fn string
			switch msg.What {
			case what.SAttr:
				fn = "setAttribute"
			case what.RAttr:
				fn = "removeAttribute"
			case what.SClass:
				fn = "classList?.add"
			case what.RClass:
				fn = "classList?.remove"
			}
			if fn != "" {
				b = append(b, "document.getElementById("...)
				b = msg.Jid.AppendQuote(b)
				b = append(b, ")?."...)
				b = append(b, fn...)
				b = append(b, "("...)
				attr, val, ok := strings.Cut(msg.Data, "\n")
				b = appendJSQuote(b, attr)
				if ok {
					b = append(b, ',')
					b = appendJSQuote(b, val)
				}
				b = append(b, ");\n"...)
			} else {
				rq.wsQueue[n] = msg
				n++
			}
		}
		for i := n; i < len(rq.wsQueue); i++ {
			rq.wsQueue[i] = wire.WsMsg{}
		}
		rq.wsQueue = rq.wsQueue[:n]
		if len(b) > 0 {
			_, err = w.Write(b)
		}
	}
	return
}

func (rq *Request) writeTailScriptResponse(w http.ResponseWriter) (err error) {
	hdr := w.Header()
	hdr["Cache-Control"] = headerCacheControlNoStore
	hdr["Content-Type"] = headerContentTypeJavaScript
	var sent bool
	if sent, err = rq.writeTailScript(w); !sent {
		w.WriteHeader(http.StatusNoContent)
	}
	return
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply HTML attribute and class updates made during initial
// rendering, which minimizes flicker without having to write the correct
// value in templates or during [Renderer.JawsRender].
//
// It also adds a <noscript> tag that warns of reduced functionality.
func (rq *Request) TailHTML(w io.Writer) (err error) {
	ks := rq.JawsKeyString()
	_, err = fmt.Fprintf(w, "\n"+`<noscript>`+
		`<div class="jaws-alert">This site requires Javascript for full functionality.</div>`+
		`<img src="/jaws/%s/noscript" alt="noscript"></noscript>`+"\n"+
		`<script src="/jaws/.tail/%s"></script>`+"\n", ks, ks)
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

// Get is shorthand for Session().Get and returns the session value associated with the key, or nil.
// If no session is associated with the [Request], it returns nil.
func (rq *Request) Get(key string) any {
	return rq.Session().Get(key)
}

// Set is shorthand for Session().Set and sets a session value to be associated with the key.
// If value is nil, the key is removed from the session.
// Does nothing if no session is associated with the [Request].
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

// SetContext atomically replaces the Request's context with the function return value.
// The function is given the current context and must return a non-nil context.
// The returned context must be derived from oldCtx so cancellation and deadlines
// continue to propagate to [Request.Context].
//
// Returning a nil context is a programming error: debug builds panic and production
// builds report it via [Jaws.MustLog] and keep the existing context.
func (rq *Request) SetContext(fn func(oldCtx context.Context) (newCtx context.Context)) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if newCtx := fn(rq.ctx); newCtx != nil {
		rq.ctx = newCtx
	} else {
		rq.Jaws.reportMisuse(errors.New("jaws: SetContext function returned a nil context"))
	}
}

// maintenance reports whether rq has expired and should be recycled. For a
// request that never went live it cancels and reports expiry once it has been
// idle longer than requestTimeout, or immediately if its context is already
// done. Called from the Serve loop's maintenance pass while jw.mu is held.
func (rq *Request) maintenance(now time.Time, requestTimeout time.Duration) bool {
	if !rq.running.Load() {
		if rq.Rendering.Swap(false) {
			rq.mu.Lock()
			rq.lastWrite = now
			rq.mu.Unlock()
		}
		rq.mu.RLock()
		err := rq.ctx.Err()
		since := now.Sub(rq.lastWrite)
		rq.mu.RUnlock()
		if err != nil {
			return true
		}
		if since > requestTimeout {
			rq.cancel(newErrNoWebSocketRequest(rq))
			return true
		}
	}
	return false
}

// cancelLocked cancels the request's context with a wrapped cause, but only for a
// live request (non-zero key) whose context has not already been cancelled.
// Caller must hold rq.mu.
func (rq *Request) cancelLocked(err error) {
	if rq.JawsKey != 0 && rq.ctx.Err() == nil {
		rq.cancelFn(rq.Jaws.Log(newErrRequestCancelledLocked(rq, err)))
	}
}

// cancel locks rq.mu and calls cancelLocked.
func (rq *Request) cancel(err error) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.cancelLocked(err)
}

// Cancel aborts the Request.
//
// It cancels the Request's context with the given cause (logged via [Jaws.Logger])
// and tears down its WebSocket processing loop. It is safe to call from UI code, for
// example to terminate a connection that violates a server-side limit. A nil err
// cancels without a specific cause, and calling Cancel on an already-finished or
// already-cancelled Request has no effect.
func (rq *Request) Cancel(err error) {
	rq.cancel(err)
}

// alertData builds the wire payload for an Alert message, HTML-escaping both the
// level and the message. Callers may therefore pass untrusted text safely; this
// mirrors the escaping done by the internal [wire.WsMsg.FillAlert] path.
func alertData(level, msg string) string {
	return html.EscapeString(level) + "\n" + html.EscapeString(msg)
}

// Alert attempts to show an alert message on the current request webpage if it has an HTML element with the id "jaws-alerts".
// The level argument should be one of Bootstrap's alert levels: primary, secondary, success, danger, warning, info, light or dark.
//
// The level and msg are HTML-escaped before being sent, so it is safe to pass
// untrusted text; do not pre-escape it.
//
// The default JaWS JavaScript only supports Bootstrap dismissible alerts.
// See [Jaws.Broadcast] for processing-loop requirements.
func (rq *Request) Alert(level, msg string) {
	rq.Jaws.Broadcast(wire.Message{
		Dest: rq,
		What: what.Alert,
		Data: alertData(level, msg),
	})
}

// AlertError calls [Request.Alert] if the given error is not nil.
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
// See [Jaws.Broadcast] for processing-loop requirements.
func (rq *Request) Redirect(url string) {
	if msg, ok := rq.Jaws.redirectMessage(url); ok {
		msg.Dest = rq
		rq.Jaws.Broadcast(msg)
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
// if elem is nil. The returned slice is a snapshot and must not be modified.
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
	case *Request:
		return dest == rq
	case string: // HTML id
		return true
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
func (rq *Request) appendDirtyTags(tags []any) {
	rq.mu.Lock()
	rq.todoDirt = append(rq.todoDirt, tags...)
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

// process is the main message processing loop. Will unsubscribe broadcastMsgCh and close outboundMsgCh on exit.
func (rq *Request) process(broadcastMsgCh chan wire.Message, incomingMsgCh <-chan wire.WsMsg, outboundMsgCh chan<- wire.WsMsg) {
	jawsDoneCh := rq.Jaws.Done()
	// Snapshot cancelFn under rq.mu, the same way ServeHTTP does: its only writers
	// (claim, getRequestLocked, clearLocked) run strictly before or after process,
	// so the captured value is stable for the loop's lifetime and the cleanup defer
	// avoids a lock-free field read.
	rq.mu.RLock()
	httpDoneCh := rq.httpDoneCh
	cancelFn := rq.cancelFn
	rq.mu.RUnlock()
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundMsgCh))
	go rq.eventCaller(eventCallCh, outboundMsgCh, eventDoneCh)

	defer func() {
		rq.Jaws.unsubscribe(broadcastMsgCh)
		rq.killSession()
		cancelFn(nil)
		close(eventCallCh)
		for {
			select {
			case _, ok := <-incomingMsgCh:
				if !ok {
					incomingMsgCh = nil
				}
			case <-eventDoneCh:
				close(outboundMsgCh)
				if x := recover(); x != nil {
					var err error
					var ok bool
					if err, ok = x.(error); !ok {
						err = fmt.Errorf("jaws: %v panic: %v", rq, x)
					}
					// Log non-fatally rather than MustLog: this runs in the cleanup
					// defer with no surrounding recover, and the panic is already
					// contained, so the request is torn down regardless.
					_ = rq.Jaws.Log(err)
				}
				return
			}
		}
	}()

	for {
		var tagmsg wire.Message
		var wsmsg wire.WsMsg
		var ok bool

		rq.sendQueue(outboundMsgCh)

		// Empty the dirty tags list and call JawsUpdate()
		// for identified elements. This queues up wsMsg's
		// in elem.wsQueue.
		for _, elem := range rq.makeUpdateList() {
			elem.JawsUpdate()
		}

		rq.sendQueue(outboundMsgCh)

		select {
		case <-jawsDoneCh:
		case <-httpDoneCh:
		case <-rq.Context().Done():
		case tagmsg, ok = <-broadcastMsgCh:
		case wsmsg, ok = <-incomingMsgCh:
			if ok {
				// incoming event message from the WebSocket
				rq.handleIncoming(wsmsg, eventCallCh)
				continue
			}
		}

		if !ok {
			// one of the channels are closed, so we're done
			return
		}

		rq.handleBroadcast(tagmsg, eventCallCh)
	}
}

// handleIncoming processes a single incoming WebSocket event message, queuing an
// event-function call or handling a child removal. Called only from process.
func (rq *Request) handleIncoming(wsmsg wire.WsMsg, eventCallCh chan eventFnCall) {
	if wsmsg.Jid.IsValid() {
		switch wsmsg.What {
		case what.Input, what.Click, what.ContextMenu, what.Set:
			rq.queueEvent(eventCallCh, eventFnCall{jid: wsmsg.Jid, wht: wsmsg.What, data: wsmsg.Data})
		case what.Remove:
			rq.handleRemove(wsmsg.Jid, wsmsg.Data)
		}
	}
}

// handleBroadcast processes a single broadcast (tag) message: it resolves the
// message destination to the affected elements and dispatches by command. Called
// only from process.
func (rq *Request) handleBroadcast(tagmsg wire.Message, eventCallCh chan eventFnCall) {
	// Reload, Redirect, Order and Alert are page-global commands: they apply to
	// the whole document and ignore element/string targeting, so emit the single
	// Jid:0 frame and return before resolving Dest. Without this early return a
	// string (HTML id) Dest would also queue a second, contradictory
	// element-targeted Jid:-1 frame for these commands.
	switch tagmsg.What {
	case what.Reload, what.Redirect, what.Order, what.Alert:
		rq.queue(wire.WsMsg{
			Jid:  0,
			Data: tagmsg.Data,
			What: tagmsg.What,
		})
		return
	}

	// collect all elements marked with the tag in the message
	var todo []*Element
	switch v := tagmsg.Dest.(type) {
	case nil:
		// matches no elements
	case *Request:
	case string:
		// target is a regular HTML ID
		data := tagmsg.Data
		if tagmsg.What != what.Set && tagmsg.What != what.Call {
			// Quote the same JSON-safe way element-targeted messages are quoted
			// (WsMsg.Append writes Jid<0 data verbatim, so this is the wire
			// quoting). strconv.Quote would emit \xNN / \UXXXXXXXX escapes that
			// the browser's JSON.parse rejects, dropping the whole frame.
			data = string(wire.AppendJSONQuote(nil, data))
		}
		rq.queue(wire.WsMsg{
			Data: v + "\t" + data,
			What: tagmsg.What,
			Jid:  -1,
		})
	default:
		todo = rq.GetElements(v)
	}

	for _, elem := range todo {
		switch tagmsg.What {
		case what.Delete:
			rq.queue(wire.WsMsg{
				Jid:  elem.Jid(),
				What: what.Delete,
			})
			rq.DeleteElement(elem)
		case what.Input, what.Click, what.ContextMenu:
			// Input, Click or ContextMenu messages received here come from broadcasts;
			// primarily used in tests by injecting a wire.WsMsg on the inbound channel.
			// they won't be sent out on the WebSocket, but will queue up a
			// call to the event function (if any).
			// primary usecase is tests.
			rq.queueEvent(eventCallCh, eventFnCall{jid: elem.Jid(), wht: tagmsg.What, data: tagmsg.Data})
		case what.Hook:
			// "hook" messages are used to synchronously call an event function.
			// the function must not send any messages itself, but may return
			// an error to be sent out as an alert message.
			// primary usecase is tests.
			if err := rq.Jaws.Log(rq.callAllEventHandlers(elem.Jid(), tagmsg.What, tagmsg.Data)); err != nil {
				var m wire.WsMsg
				m.FillAlert(err)
				m.Jid = elem.Jid()
				rq.queue(m)
			}
		case what.Update:
			elem.JawsUpdate()
		default:
			rq.queue(wire.WsMsg{
				Data: tagmsg.Data,
				Jid:  elem.Jid(),
				What: tagmsg.What,
			})
		}
	}
}

func (rq *Request) handleRemove(containerJid Jid, data string) {
	// For incoming what.Remove from jaws.js, Data is a tab-separated list of
	// managed descendant IDs that were removed. The WebSocket Jid identifies the
	// parent/container in the DOM and must not itself be deleted here.
	//
	// The client is already trusted only within its own request: a malicious client
	// can fully control the DOM and UI it presents to its user. Treating arbitrary
	// child removals as request-local state cleanup is therefore not a server-side
	// privilege boundary; IDs are only looked up in this Request.
	if containerJid > 0 {
		rq.mu.Lock()
		defer rq.mu.Unlock()
		// Collect the requested child elements, then delete them in a single pass
		// over rq.elems and rq.tagMap, rather than an O(N) scan plus O(N) compaction
		// per id (the id count is client-controlled, bounded by the read limit).
		var victims map[Jid]struct{}
		for jidstr := range strings.SplitSeq(data, "\t") {
			if e := rq.getElementByJidLocked(jid.ParseString(jidstr)); e != nil {
				if victims == nil {
					victims = map[Jid]struct{}{}
				}
				e.deleted.Store(true)
				victims[e.Jid()] = struct{}{}
			}
		}
		if len(victims) == 0 {
			return
		}
		isVictim := func(e *Element) bool { _, ok := victims[e.Jid()]; return ok }
		rq.elems = slices.DeleteFunc(rq.elems, isVictim)
		for k := range rq.tagMap {
			rq.tagMap[k] = slices.DeleteFunc(rq.tagMap[k], isVictim)
			if len(rq.tagMap[k]) == 0 {
				delete(rq.tagMap, k)
			}
		}
	}
}

// queue appends a single outbound message to the request's pending wsQueue under
// muQueue, the leaf lock that orders writes independently of rq.mu. The Serve
// loop later drains it via getSendMsgs.
func (rq *Request) queue(msg wire.WsMsg) {
	rq.muQueue.Lock()
	rq.wsQueue = append(rq.wsQueue, msg)
	rq.muQueue.Unlock()
}

// callAllEventHandlers dispatches a single incoming event to the target
// element(s) and returns the first result that is not ErrEventUnhandled. A zero
// id with a Click or ContextMenu carries a tab-separated list of bubbled element
// jids, which are resolved and tried in order; any other id resolves to a single
// element. ErrEventUnhandled is normalized to nil. rq.mu is held only for the
// element lookups; the handlers themselves run unlocked.
func (rq *Request) callAllEventHandlers(id Jid, wht what.What, value string) (err error) {
	var elems []*Element
	rq.mu.RLock()
	if id == 0 {
		if wht == what.Click || wht == what.ContextMenu {
			var after string
			var found bool
			value, after, found = strings.Cut(value, "\t")
			for found {
				var jidStr string
				jidStr, after, found = strings.Cut(after, "\t")
				if id = jid.ParseString(jidStr); id > 0 {
					if e := rq.getElementByJidLocked(id); e != nil && !e.deleted.Load() {
						elems = append(elems, e)
					}
				}
			}
		}
	} else {
		if e := rq.getElementByJidLocked(id); e != nil && !e.deleted.Load() {
			elems = append(elems, e)
		}
	}
	rq.mu.RUnlock()

	for _, e := range elems {
		if err = CallEventHandlers(e.UI(), e, wht, value); !errors.Is(err, ErrEventUnhandled) {
			return
		}
	}
	if errors.Is(err, ErrEventUnhandled) {
		err = nil
	}
	return
}

// queueEvent hands a resolved event-function call to the eventCaller goroutine.
//
// eventCallCh is buffered to the outbound capacity; if it is full the request has
// fallen too far behind to stay consistent (an event would be lost), so it is
// cancelled rather than dropping the event, mirroring the broadcast back-pressure
// path in [Jaws.ServeWithTimeout]. cancel takes rq.mu, which the process loop does
// not hold when calling this.
func (rq *Request) queueEvent(eventCallCh chan eventFnCall, call eventFnCall) {
	select {
	case eventCallCh <- call:
	default:
		rq.cancel(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, call))
	}
}

// getSendMsgs drains the pending wsQueue, dropping messages addressed to elements
// that are not present (non-element messages and Delete are always kept), and
// returns the survivors sorted by Jid. It takes rq.mu (read) then muQueue, the
// order required by the lock hierarchy documented in jaws.go.
func (rq *Request) getSendMsgs() (toSend []wire.WsMsg) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()

	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	if len(rq.wsQueue) > 0 {
		// validJids is built lazily and at most once: only messages addressed to a
		// specific element (Jid >= 1, not Delete) need it, so an idle drain — the
		// common case on the process loop's hot path — allocates nothing. Holding
		// rq.mu (read) keeps rq.elems stable while the map is built.
		var validJids map[Jid]struct{}
		for i := range rq.wsQueue {
			ok := rq.wsQueue[i].Jid < 1 || rq.wsQueue[i].What == what.Delete
			if !ok {
				if validJids == nil {
					validJids = make(map[Jid]struct{}, len(rq.elems))
					for _, elem := range rq.elems {
						if !elem.deleted.Load() {
							validJids[elem.Jid()] = struct{}{}
						}
					}
				}
				_, ok = validJids[rq.wsQueue[i].Jid]
			}
			if ok {
				toSend = append(toSend, rq.wsQueue[i])
			}
		}
		rq.wsQueue = rq.wsQueue[:0]
	}

	slices.SortStableFunc(toSend, func(a, b wire.WsMsg) int { return cmp.Compare(a.Jid, b.Jid) })
	return
}

// sendQueue writes the drained outbound queue to outboundMsgCh, abandoning a send
// if the request context is cancelled.
func (rq *Request) sendQueue(outboundMsgCh chan<- wire.WsMsg) {
	msgs := rq.getSendMsgs()
	if len(msgs) == 0 {
		return
	}
	// Snapshot the done channel once for the whole batch. getSendMsgs already
	// froze the batch under a single lock, and rq.Context() takes rq.mu.RLock on
	// every call, so reading it per message would re-lock rq.mu K times during a
	// drain burst. A SetContext mid-drain is already unsynchronized relative to an
	// in-flight send, so capturing once changes no guaranteed behavior.
	done := rq.Context().Done()
	for _, msg := range msgs {
		select {
		case <-done:
		case outboundMsgCh <- msg:
		}
	}
}

// deleteElementLocked removes elem from the request's element list and from every
// tag entry, marking it deleted; it is a no-op if elem belongs to another request.
// Caller must hold rq.mu.
func (rq *Request) deleteElementLocked(elem *Element) {
	if elem.Request == rq {
		elem.deleted.Store(true)
		// slices.DeleteFunc removes every match and zeros the freed tail slots, so
		// the dropped *Element pointers do not linger in the backing array.
		isElem := func(e *Element) bool { return e == elem }
		rq.elems = slices.DeleteFunc(rq.elems, isElem)
		for k := range rq.tagMap {
			rq.tagMap[k] = slices.DeleteFunc(rq.tagMap[k], isElem)
			if len(rq.tagMap[k]) == 0 {
				delete(rq.tagMap, k)
			}
		}
	}
}

// DeleteElement removes elem from the [Request] element registry.
//
// This is primarily intended for UI implementations that manage dynamic child
// element sets and need to drop stale elements after issuing a corresponding
// DOM remove operation.
func (rq *Request) DeleteElement(elem *Element) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.deleteElementLocked(elem)
}

// makeUpdateList drains the pending-dirt tag list, resolves it to the distinct
// elements needing an update, clears the list, and returns those elements sorted
// by Jid. It takes rq.mu. The Serve loop calls JawsUpdate on each returned element.
func (rq *Request) makeUpdateList() (todo []*Element) {
	rq.mu.Lock()
	seen := map[*Element]struct{}{}
	for _, tagValue := range rq.todoDirt {
		for _, elem := range rq.tagMap[tagValue] {
			if _, ok := seen[elem]; !ok {
				seen[elem] = struct{}{}
				todo = append(todo, elem)
			}
		}
	}
	clear(rq.todoDirt)
	rq.todoDirt = rq.todoDirt[:0]
	rq.mu.Unlock()
	slices.SortFunc(todo, func(a, b *Element) int { return cmp.Compare(a.Jid(), b.Jid()) })
	return
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundMsgCh chan<- wire.WsMsg, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		select {
		case <-rq.Context().Done():
			continue
		default:
		}
		if err := rq.callAllEventHandlers(call.jid, call.wht, call.data); err != nil {
			var m wire.WsMsg
			m.FillAlert(err)
			select {
			case outboundMsgCh <- m:
			default:
				_ = rq.Jaws.Log(fmt.Errorf("jaws: outboundMsgCh full sending event error '%s'", err.Error()))
			}
		}
	}
}

// onConnect calls the [Request]'s [ConnectFn] if it is not nil, and returns the error from it.
// Returns nil if [ConnectFn] is nil.
func (rq *Request) onConnect() (err error) {
	rq.mu.RLock()
	connectFn := rq.connectFn
	rq.mu.RUnlock()
	if connectFn != nil {
		err = connectFn(rq)
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
// pass, which also holds jw.mu when deciding whether to recycle a not-running
// request. If the request was recycled first, clearLocked has reset claimed, so the
// CAS below fails and ServeHTTP returns Gone instead of driving a dead request.
func (rq *Request) startServe() (ok bool) {
	rq.Jaws.mu.Lock()
	defer rq.Jaws.mu.Unlock()
	return rq.claimed.Load() && rq.running.CompareAndSwap(false, true)
}

func (rq *Request) stopServe() {
	rq.cancel(nil)
	rq.Jaws.recycle(rq)
}

var headerContentTypeJavaScript = []string{"text/javascript"}

// ServeHTTP implements [http.Handler].
//
// Requires [Jaws.UseRequest] to have been successfully called for the [Request].
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
