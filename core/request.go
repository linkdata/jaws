package core

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
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

// ConnectFn can be used to interact with a Request before message processing starts.
// Returning an error causes the Request to abort, and the WebSocket connection to close.
type ConnectFn = func(rq *Request) error

// Request maintains the state for a JaWS WebSocket connection, and handles processing
// of events and broadcasts.
//
// Note that we have to store the context inside the struct because there is no call chain
// between the Request being created and it being used once the WebSocket is created.
type Request struct {
	Jaws       *Jaws                   // (read-only) the JaWS instance the Request belongs to
	JawsKey    uint64                  // (read-only) a random number used in the WebSocket URI to identify this Request
	remoteIP   netip.Addr              // (read-only) remote IP, or nil
	Rendering  atomic.Bool             // set to true by RequestWriter.Write()
	running    atomic.Bool             // if ServeHTTP() is running
	claimed    atomic.Bool             // if UseRequest() has been called for it
	mu         deadlock.RWMutex        // protects following
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
	muQueue    deadlock.Mutex          // protects wsQueue
	wsQueue    []WsMsg                 // queued messages to send
}

type eventFnCall struct {
	jid  Jid
	wht  what.What
	data string
}

var (
	ErrWebsocketOriginMissing     = errors.New("websocket request missing Origin header")
	ErrWebsocketOriginWrongScheme = errors.New("websocket Origin not http or https")
	ErrWebsocketOriginWrongHost   = errors.New("websocket Origin host mismatch")
	ErrRequestAlreadyClaimed      = errors.New("request already claimed")
	ErrJavascriptDisabled         = errors.New("javascript is disabled")
)

func (rq *Request) JawsKeyString() string {
	jawsKey := uint64(0)
	if rq != nil {
		jawsKey = rq.JawsKey
	}
	return JawsKeyString(jawsKey)
}

func (rq *Request) String() string {
	return "Request<" + rq.JawsKeyString() + ">"
}

func (rq *Request) claim(hr *http.Request) error {
	if !rq.claimed.Load() {
		var actualIP netip.Addr
		var httpDoneCh <-chan struct{}
		if hr != nil { // can be nil in tests
			actualIP = parseIP(hr.RemoteAddr)
			httpDoneCh = hr.Context().Done()
		}
		rq.mu.Lock()
		defer rq.mu.Unlock()
		if !equalIP(rq.remoteIP, actualIP) {
			return fmt.Errorf("/jaws/%s: expected IP %q, got %q", rq.JawsKeyString(), rq.remoteIP.String(), actualIP.String())
		}
		if rq.claimed.CompareAndSwap(false, true) {
			rq.ctx, rq.cancelFn = context.WithCancelCause(rq.ctx)
			rq.httpDoneCh = httpDoneCh
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

func (rq *Request) clearLocked() *Request {
	rq.JawsKey = 0
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
	rq.todoDirt = rq.todoDirt[:0]
	rq.remoteIP = netip.Addr{}
	for _, e := range rq.elems {
		if e != nil {
			e.Request = nil
			e.handlers = nil
			e.ui = nil
		}
	}
	clear(rq.elems)
	rq.elems = rq.elems[:0]
	rq.wsQueue = rq.wsQueue[:0]
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
	b = JawsKeyAppend(b, rq.JawsKey)
	b = append(b, `">`...)
	_, err = w.Write(b)
	return
}

// appendJSQuote is like strconv.AppendQuote but also escapes '<' as '\x3c'
// to prevent '</script>' from closing the script block when embedded in HTML.
func appendJSQuote(b []byte, s string) []byte {
	quoted := strconv.AppendQuote(nil, s)
	return append(b, bytes.ReplaceAll(quoted, []byte("<"), []byte(`\x3c`))...)
}

func (rq *Request) getTailActions() (b []byte) {
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
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
			if len(b) == 0 {
				b = append(b, "\n<script>"...)
			}
			b = append(b, "\ndocument.getElementById("...)
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
			b = append(b, ");"...)
		} else {
			rq.wsQueue[n] = msg
			n++
		}
	}
	for i := n; i < len(rq.wsQueue); i++ {
		rq.wsQueue[i] = WsMsg{}
	}
	rq.wsQueue = rq.wsQueue[:n]
	if len(b) > 0 {
		b = append(b, "\n</script>"...)
	}
	return
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply HTML attribute and class updates made during initial
// rendering, which eliminates flicker without having to write the correct
// value in templates or during JawsRender().
//
// It also adds a <noscript> tag that warns of reduces functionality.
func (rq *Request) TailHTML(w io.Writer) (err error) {
	if _, err = fmt.Fprintf(w, "\n"+`<noscript>`+
		`<div class="jaws-alert">This site requires Javascript for full functionality.</div>`+
		`<img src="/jaws/%s/noscript" alt="noscript"></noscript>`, rq.JawsKeyString()); err == nil {
		if actions := rq.getTailActions(); len(actions) > 0 {
			_, err = w.Write(actions)
		}
	}
	return
}

// GetConnectFn returns the currently set ConnectFn. That function will be called before starting the WebSocket tunnel if not nil.
func (rq *Request) GetConnectFn() (fn ConnectFn) {
	rq.mu.RLock()
	fn = rq.connectFn
	rq.mu.RUnlock()
	return
}

// SetConnectFn sets ConnectFn. That function will be called before starting the WebSocket tunnel if not nil.
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

// Get is shorthand for `Session().Get()` and returns the session value associated with the key, or nil.
// It no session is associated with the Request, returns nil.
func (rq *Request) Get(key string) any {
	return rq.Session().Get(key)
}

// Set is shorthand for `Session().Set()` and sets a session value to be associated with the key.
// If value is nil, the key is removed from the session.
// Does nothing if there is no session is associated with the Request.
func (rq *Request) Set(key string, val any) {
	rq.Session().Set(key, val)
}

// Context returns the Request's Context, which is by default derived from jaws.BaseContext.
func (rq *Request) Context() (ctx context.Context) {
	rq.mu.RLock()
	ctx = rq.ctx
	rq.mu.RUnlock()
	return
}

// SetContext atomically replaces the Request's context with the function return value.
// The function is given the current context and must return a non-nil context.
// The returned context must be derived from oldctx so cancellation and deadlines
// continue to propagate to Request.Context().
func (rq *Request) SetContext(fn func(oldctx context.Context) (newctx context.Context)) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if rq.ctx = fn(rq.ctx); rq.ctx == nil {
		panic("context must not be nil")
	}
}

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

func (rq *Request) cancelLocked(err error) {
	if rq.JawsKey != 0 && rq.ctx.Err() == nil {
		if !rq.running.Load() {
			err = newErrPendingCancelledLocked(rq, err)
		}
		rq.cancelFn(rq.Jaws.Log(err))
	}
}

func (rq *Request) cancel(err error) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.cancelLocked(err)
}

// Alert attempts to show an alert message on the current request webpage if it has an HTML element with the id 'jaws-alert'.
// The lvl argument should be one of Bootstraps alert levels: primary, secondary, success, danger, warning, info, light or dark.
//
// The default JaWS javascript only supports Bootstrap.js dismissable alerts.
// See Jaws.Broadcast for processing-loop requirements.
func (rq *Request) Alert(lvl, msg string) {
	rq.Jaws.Broadcast(Message{
		Dest: rq,
		What: what.Alert,
		Data: lvl + "\n" + msg,
	})
}

// AlertError calls Alert if the given error is not nil.
func (rq *Request) AlertError(err error) {
	if rq.Jaws.Log(err) != nil {
		rq.Alert("danger", html.EscapeString(err.Error()))
	}
}

// Redirect requests the current Request to navigate to the given URL.
// See Jaws.Broadcast for processing-loop requirements.
func (rq *Request) Redirect(url string) {
	rq.Jaws.Broadcast(Message{
		Dest: rq,
		What: what.Redirect,
		Data: url,
	})
}

func (rq *Request) tagsOfLocked(elem *Element) (tags []any) {
	for tag, elems := range rq.tagMap {
		for _, e := range elems {
			if e == elem {
				tags = append(tags, tag)
				break
			}
		}
	}
	return
}

func (rq *Request) TagsOf(elem *Element) (tags []any) {
	if elem != nil {
		rq.mu.RLock()
		defer rq.mu.RUnlock()
		tags = rq.tagsOfLocked(elem)
	}
	return
}

// Dirty marks all Elements that have one or more of the given tags as dirty.
func (rq *Request) Dirty(tags ...any) {
	rq.Jaws.setDirty(MustTagExpand(rq, tags))
}

// wantMessage returns true if the Request want the message.
func (rq *Request) wantMessage(msg *Message) (yes bool) {
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
		_, yes = rq.tagMap[msg.Dest]
		rq.mu.RUnlock()
	}
	return
}

// NextJid is the next Jid that should be used. Used when testing. Do not modify it outside of tests.
var NextJid Jid

func (rq *Request) newElementLocked(ui UI) (elem *Element) {
	elem = &Element{
		jid:     Jid(atomic.AddInt64((*int64)(&NextJid), 1)),
		ui:      ui,
		Request: rq,
	}
	rq.elems = append(rq.elems, elem)
	return
}

// NewElement creates a new Element using the given UI object.
//
// Panics if the build tag "debug" is set and the UI object doesn't satisfy all requirements.
func (rq *Request) NewElement(ui UI) *Element {
	if deadlock.Debug {
		if err := newErrNotComparable(ui); err != nil {
			panic(err)
		}
	}
	rq.mu.Lock()
	defer rq.mu.Unlock()
	return rq.newElementLocked(ui)
}

func (rq *Request) GetElementByJid(jid Jid) (e *Element) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	e = rq.getElementByJidLocked(jid)
	return
}

func (rq *Request) getElementByJidLocked(jid Jid) (elem *Element) {
	for _, e := range rq.elems {
		if e.Jid() == jid {
			elem = e
			break
		}
	}
	return
}

func (rq *Request) hasTagLocked(elem *Element, tag any) bool {
	for _, e := range rq.tagMap[tag] {
		if elem == e {
			return true
		}
	}
	return false
}

func (rq *Request) HasTag(elem *Element, tag any) (yes bool) {
	rq.mu.RLock()
	yes = rq.hasTagLocked(elem, tag)
	rq.mu.RUnlock()
	return
}

func (rq *Request) appendDirtyTags(tags []any) {
	rq.mu.Lock()
	rq.todoDirt = append(rq.todoDirt, tags...)
	rq.mu.Unlock()
}

// Tag adds the given tags to the given Element.
func (rq *Request) TagExpanded(elem *Element, expandedtags []any) {
	if elem != nil && !elem.deleted.Load() && elem.Request == rq {
		rq.mu.Lock()
		defer rq.mu.Unlock()
		for _, tag := range expandedtags {
			if !rq.hasTagLocked(elem, tag) {
				rq.tagMap[tag] = append(rq.tagMap[tag], elem)
			}
		}
	}
}

// Tag adds the given tags to the given Element.
func (rq *Request) Tag(elem *Element, tags ...any) {
	if elem != nil && len(tags) > 0 && elem.Request == rq {
		rq.TagExpanded(elem, MustTagExpand(elem.Request, tags))
	}
}

// GetElements returns a list of the UI elements in the Request that have the given tag(s).
func (rq *Request) GetElements(tagitem any) (elems []*Element) {
	tags := MustTagExpand(rq, tagitem)
	seen := map[*Element]struct{}{}
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	for _, tag := range tags {
		if el, ok := rq.tagMap[tag]; ok {
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
func (rq *Request) process(broadcastMsgCh chan Message, incomingMsgCh <-chan WsMsg, outboundMsgCh chan<- WsMsg) {
	jawsDoneCh := rq.Jaws.Done()
	httpDoneCh := rq.httpDoneCh
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundMsgCh))
	go rq.eventCaller(eventCallCh, outboundMsgCh, eventDoneCh)

	defer func() {
		rq.Jaws.unsubscribe(broadcastMsgCh)
		rq.killSession()
		rq.cancelFn(nil)
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
					rq.Jaws.MustLog(err)
				}
				return
			}
		}
	}()

	for {
		var tagmsg Message
		var wsmsg WsMsg
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
				// incoming event message from the websocket
				if wsmsg.Jid.IsValid() {
					switch wsmsg.What {
					case what.Input, what.Click, what.Set:
						rq.queueEvent(eventCallCh, eventFnCall{jid: wsmsg.Jid, wht: wsmsg.What, data: wsmsg.Data})
					case what.Remove:
						rq.handleRemove(wsmsg.Data)
					}
				}
				continue
			}
		}

		if !ok {
			// one of the channels are closed, so we're done
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
				data = strconv.Quote(data)
			}
			rq.queue(WsMsg{
				Data: v + "\t" + data,
				What: tagmsg.What,
				Jid:  -1,
			})
		default:
			todo = rq.GetElements(v)
		}

		switch tagmsg.What {
		case what.Reload, what.Redirect, what.Order, what.Alert:
			rq.queue(WsMsg{
				Jid:  0,
				Data: tagmsg.Data,
				What: tagmsg.What,
			})
		default:
			for _, elem := range todo {
				switch tagmsg.What {
				case what.Delete:
					rq.queue(WsMsg{
						Jid:  elem.Jid(),
						What: what.Delete,
					})
					rq.DeleteElement(elem)
				case what.Input, what.Click:
					// Input or Click messages received here are from Request.Send() or broadcasts.
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
						var m WsMsg
						m.FillAlert(err)
						m.Jid = elem.Jid()
						rq.queue(m)
					}
				case what.Update:
					elem.JawsUpdate()
				default:
					rq.queue(WsMsg{
						Data: tagmsg.Data,
						Jid:  elem.Jid(),
						What: tagmsg.What,
					})
				}
			}
		}
	}
}

func (rq *Request) handleRemove(data string) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	for _, jidstr := range strings.Split(data, "\t") {
		if e := rq.getElementByJidLocked(jid.ParseString(jidstr)); e != nil {
			rq.deleteElementLocked(e)
		}
	}
}

func (rq *Request) queue(msg WsMsg) {
	rq.muQueue.Lock()
	rq.wsQueue = append(rq.wsQueue, msg)
	rq.muQueue.Unlock()
}

func (rq *Request) callAllEventHandlers(id Jid, wht what.What, val string) (err error) {
	var elems []*Element
	rq.mu.RLock()
	if id == 0 {
		if wht == what.Click {
			var after string
			var found bool
			val, after, found = strings.Cut(val, "\t")
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
		if err = CallEventHandlers(e.Ui(), e, wht, val); err != ErrEventUnhandled {
			return
		}
	}
	if err == ErrEventUnhandled {
		err = nil
	}
	return
}

func (rq *Request) queueEvent(eventCallCh chan eventFnCall, call eventFnCall) {
	select {
	case eventCallCh <- call:
	default:
		rq.Jaws.MustLog(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, call))
		return
	}
}

func (rq *Request) getSendMsgs() (toSend []WsMsg) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()

	validJids := map[Jid]struct{}{}
	for _, elem := range rq.elems {
		if !elem.deleted.Load() {
			validJids[elem.Jid()] = struct{}{}
		}
	}

	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	if len(rq.wsQueue) > 0 {
		for i := range rq.wsQueue {
			ok := rq.wsQueue[i].Jid < 1 || rq.wsQueue[i].What == what.Delete
			if !ok {
				_, ok = validJids[rq.wsQueue[i].Jid]
			}
			if ok {
				toSend = append(toSend, rq.wsQueue[i])
			}
		}
		rq.wsQueue = rq.wsQueue[:0]
	}

	slices.SortStableFunc(toSend, func(a, b WsMsg) int { return cmp.Compare(a.Jid, b.Jid) })
	return
}

func (rq *Request) sendQueue(outboundMsgCh chan<- WsMsg) {
	for _, msg := range rq.getSendMsgs() {
		select {
		case <-rq.Context().Done():
		case outboundMsgCh <- msg:
		}
	}
}

func deleteElement(s []*Element, e *Element) []*Element {
	for i, v := range s {
		if e == v {
			j := i
			for i++; i < len(s); i++ {
				v = s[i]
				if e != v {
					s[j] = v
					j++
				}
			}
			for i := j; i < len(s); i++ {
				s[i] = nil
			}
			return s[:j]
		}
	}
	return s
}

func (rq *Request) deleteElementLocked(e *Element) {
	if e.Request == rq {
		e.deleted.Store(true)
		rq.elems = deleteElement(rq.elems, e)
		for k := range rq.tagMap {
			rq.tagMap[k] = deleteElement(rq.tagMap[k], e)
			if len(rq.tagMap[k]) == 0 {
				delete(rq.tagMap, k)
			}
		}
	}
}

// DeleteElement removes elem from the Request element registry.
//
// This is primarily intended for UI implementations that manage dynamic child
// element sets and need to drop stale elements after issuing a corresponding
// DOM remove operation.
func (rq *Request) DeleteElement(elem *Element) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.deleteElementLocked(elem)
}

func (rq *Request) makeUpdateList() (todo []*Element) {
	rq.mu.Lock()
	seen := map[*Element]struct{}{}
	for _, tag := range rq.todoDirt {
		for _, elem := range rq.tagMap[tag] {
			if _, ok := seen[elem]; !ok {
				seen[elem] = struct{}{}
				todo = append(todo, elem)
			}
		}
	}
	clear(rq.todoDirt)
	rq.todoDirt = rq.todoDirt[:0]
	rq.mu.Unlock()
	sort.Slice(todo, func(i, j int) bool { return todo[i].Jid() < todo[j].Jid() })
	return
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundMsgCh chan<- WsMsg, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		select {
		case <-rq.Context().Done():
			continue
		default:
		}
		if err := rq.callAllEventHandlers(call.jid, call.wht, call.data); err != nil {
			var m WsMsg
			m.FillAlert(err)
			select {
			case outboundMsgCh <- m:
			default:
				_ = rq.Jaws.Log(fmt.Errorf("jaws: outboundMsgCh full sending event error '%s'", err.Error()))
			}
		}
	}
}

// onConnect calls the Request's ConnectFn if it's not nil, and returns the error from it.
// Returns nil if ConnectFn is nil.
func (rq *Request) onConnect() (err error) {
	rq.mu.RLock()
	connectFn := rq.connectFn
	rq.mu.RUnlock()
	if connectFn != nil {
		err = connectFn(rq)
	}
	return
}

func (rq *Request) validateWebSocketOrigin(r *http.Request) (err error) {
	err = ErrWebsocketOriginMissing
	if origin := r.Header.Get("Origin"); origin != "" {
		var u *url.URL
		if u, err = url.Parse(origin); err == nil {
			if initial := rq.Initial(); initial != nil {
				secure := requestIsSecure(initial)
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

// MustLog sends an error to the Logger set in the Jaws or
// panics with the given error if no Logger is set.
// Has no effect if the err is nil.
func (rq *Request) MustLog(err error) {
	var jw *Jaws
	if rq != nil {
		jw = rq.Jaws
	}
	jw.MustLog(err)
}
