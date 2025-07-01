package jaws

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/netip"
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
	rendering  atomic.Bool             // set to true by RequestWriter.Write()
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
	wsQueue    []wsMsg                 // queued messages to send
}

type eventFnCall struct {
	jid  Jid
	wht  what.What
	data string
}

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

var ErrRequestAlreadyClaimed = errors.New("request already claimed")
var ErrJavascriptDisabled = errors.New("javascript is disabled")

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
	rq.ctx, rq.cancelFn = context.WithCancelCause(rq.Jaws.BaseContext)
	rq.httpDoneCh = nil
	rq.todoDirt = rq.todoDirt[:0]
	rq.remoteIP = netip.Addr{}
	rq.elems = rq.elems[:0]
	rq.killSessionLocked()
	clear(rq.tagMap)
	return rq
}

// HeadHTML writes the HTML code needed in the HTML page's HEAD section.
func (rq *Request) HeadHTML(w io.Writer) (err error) {
	var b, jk []byte
	jk = JawsKeyAppend(jk, rq.JawsKey)
	b = append(b, rq.Jaws.headPrefix...)
	b = append(b, jk...)
	b = append(b, `";</script><noscript>`+
		`<div class="jaws-alert">This site requires Javascript for full functionality.</div>`+
		`<img src="/jaws/`...)
	b = append(b, jk...)
	b = append(b, `/noscript"></noscript>`...)
	_, err = w.Write(b)
	return
}

func (rq *Request) getTailActions() (b []byte) {
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
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
			b = strconv.AppendQuote(b, attr)
			if ok {
				b = append(b, ',')
				b = strconv.AppendQuote(b, val)
			}
			b = append(b, ");"...)
		}
	}
	if len(b) > 0 {
		b = append(b, "\n</script>"...)
	}
	return
}

// TailHTML writes optional HTML code at the end of the page's BODY section that
// will immediately apply HTML attribute and class updates made during initial
// rendering, which eliminates flicker without having to write the correct
// value in templates or during JawsRender().
func (rq *Request) TailHTML(w io.Writer) (err error) {
	if actions := rq.getTailActions(); len(actions) > 0 {
		_, err = w.Write(actions)
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
func (rq *Request) SetContext(fn func(oldctx context.Context) (newctx context.Context)) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.ctx = fn(rq.ctx)
}

func (rq *Request) maintenance(now time.Time, requestTimeout time.Duration) bool {
	if !rq.running.Load() {
		if rq.rendering.Swap(false) {
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
		yes = dest == rq
	case string: // HTML id
		yes = true
	case []any: // more than one tag
		yes = true
	default:
		rq.mu.RLock()
		_, yes = rq.tagMap[msg.Dest]
		rq.mu.RUnlock()
	}
	return
}

var nextJid Jid

func (rq *Request) newElementLocked(ui UI) (elem *Element) {
	elem = &Element{
		jid:     Jid(atomic.AddInt64((*int64)(&nextJid), 1)),
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

func (rq *Request) getElementByJidLocked(jid Jid) (elem *Element) {
	for _, e := range rq.elems {
		if e.Jid() == jid {
			elem = e
			break
		}
	}
	return
}

func (rq *Request) getElementByJid(jid Jid) (e *Element) {
	rq.mu.RLock()
	e = rq.getElementByJidLocked(jid)
	rq.mu.RUnlock()
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
func (rq *Request) tagExpanded(elem *Element, expandedtags []any) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	for _, tag := range expandedtags {
		if !rq.hasTagLocked(elem, tag) {
			rq.tagMap[tag] = append(rq.tagMap[tag], elem)
		}
	}
}

// Tag adds the given tags to the given Element.
func (rq *Request) Tag(elem *Element, tags ...any) {
	if elem != nil && len(tags) > 0 && elem.Request == rq {
		rq.tagExpanded(elem, MustTagExpand(elem.Request, tags))
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
					elems = append(elems, el...)
				}
			}
		}
	}
	return
}

// process is the main message processing loop. Will unsubscribe broadcastMsgCh and close outboundMsgCh on exit.
func (rq *Request) process(broadcastMsgCh chan Message, incomingMsgCh <-chan wsMsg, outboundMsgCh chan<- wsMsg) {
	jawsDoneCh := rq.Jaws.Done()
	httpDoneCh := rq.httpDoneCh
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundMsgCh))
	go rq.eventCaller(eventCallCh, outboundMsgCh, eventDoneCh)

	defer func() {
		rq.Jaws.unsubscribe(broadcastMsgCh)
		rq.killSession()
		close(eventCallCh)
		for {
			select {
			case <-eventCallCh:
			case <-incomingMsgCh:
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
		var wsmsg wsMsg
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
			rq.queue(wsMsg{
				Data: v + "\t" + strconv.Quote(tagmsg.Data),
				What: tagmsg.What,
				Jid:  -1,
			})
		default:
			todo = rq.GetElements(v)
		}

		switch tagmsg.What {
		case what.Reload, what.Redirect, what.Order, what.Alert:
			rq.queue(wsMsg{
				Jid:  0,
				Data: tagmsg.Data,
				What: tagmsg.What,
			})
		default:
			for _, elem := range todo {
				switch tagmsg.What {
				case what.Delete:
					rq.queue(wsMsg{
						Jid:  elem.Jid(),
						What: what.Delete,
					})
					rq.deleteElement(elem)
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
						rq.queue(wsMsg{
							Data: tagmsg.Data,
							Jid:  elem.Jid(),
							What: what.Alert,
						})
					}
				case what.Update:
					elem.JawsUpdate()
				default:
					rq.queue(wsMsg{
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

func (rq *Request) queue(msg wsMsg) {
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
					if e := rq.getElementByJidLocked(id); e != nil && !e.deleted {
						elems = append(elems, e)
					}
				}
			}
		}
	} else {
		if e := rq.getElementByJidLocked(id); e != nil && !e.deleted {
			elems = append(elems, e)
		}
	}
	rq.mu.RUnlock()

	for _, e := range elems {
		if err = callEventHandlers(e.Ui(), e, wht, val); err != ErrEventUnhandled {
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

func (rq *Request) getSendMsgs() (toSend []wsMsg) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()

	validJids := map[Jid]struct{}{}
	for _, elem := range rq.elems {
		if !elem.deleted {
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

	slices.SortStableFunc(toSend, func(a, b wsMsg) int { return int(a.Jid - b.Jid) })
	return
}

func (rq *Request) sendQueue(outboundMsgCh chan<- wsMsg) {
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
	e.deleted = true
	rq.elems = deleteElement(rq.elems, e)
	for k := range rq.tagMap {
		rq.tagMap[k] = deleteElement(rq.tagMap[k], e)
	}
}

func (rq *Request) deleteElement(e *Element) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.deleteElementLocked(e)
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

func (rq *Request) getLastWrite() (when time.Time) {
	rq.mu.RLock()
	when = rq.lastWrite
	rq.mu.RUnlock()
	return
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundMsgCh chan<- wsMsg, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		if err := rq.callAllEventHandlers(call.jid, call.wht, call.data); err != nil {
			var m wsMsg
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

// Writer returns a RequestWriter with this Request and the given Writer.
func (rq *Request) Writer(w io.Writer) RequestWriter {
	return RequestWriter{rq: rq, Writer: w}
}
