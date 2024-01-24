package jaws

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/netip"
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
	Jaws      *Jaws                   // (read-only) the JaWS instance the Request belongs to
	JawsKey   uint64                  // (read-only) a random number used in the WebSocket URI to identify this Request
	Created   time.Time               // (read-only) when the Request was created, used for automatic cleanup
	Initial   *http.Request           // (read-only) initial HTTP request passed to Jaws.NewRequest
	remoteIP  netip.Addr              // (read-only) remote IP, or nil
	session   *Session                // (read-only) session, if established
	mu        deadlock.RWMutex        // protects following
	claimed   bool                    // if UseRequest() has been called for it
	running   bool                    // if ServeHTTP() is running
	todoDirt  []interface{}           // dirty tags
	ctx       context.Context         // current context, derived from either Jaws or WS HTTP req
	cancelFn  context.CancelCauseFunc // cancel function
	connectFn ConnectFn               // a ConnectFn to call before starting message processing for the Request
	elems     []*Element
	tagMap    map[interface{}][]*Element
}

type eventFnCall struct {
	jid  Jid
	wht  what.What
	data string
}

const maxWsQueueLengthPerElement = 20

var ErrWebsocketQueueOverflow = errors.New("websocket queue overflow")

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

func (rq *Request) claim(hr *http.Request) (err error) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if rq.claimed {
		return ErrRequestAlreadyClaimed
	}
	var actualIP netip.Addr
	ctx := context.Background()
	if hr != nil {
		actualIP = parseIP(hr.RemoteAddr)
		ctx = hr.Context()
	}
	if equalIP(rq.remoteIP, actualIP) {
		rq.ctx, rq.cancelFn = context.WithCancelCause(ctx)
		rq.claimed = true
	} else {
		err = fmt.Errorf("/jaws/%s: expected IP %q, got %q", rq.JawsKeyString(), rq.remoteIP.String(), actualIP.String())
	}
	return
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
	rq.Created = time.Time{}
	rq.Initial = nil
	rq.claimed = false
	rq.running = false
	rq.ctx, rq.cancelFn = context.WithCancelCause(context.Background())
	rq.todoDirt = rq.todoDirt[:0]
	rq.remoteIP = netip.Addr{}
	rq.elems = rq.elems[:0]
	rq.killSessionLocked()
	clear(rq.tagMap)
	return rq
}

// HeadHTML writes the HTML code needed in the HTML page's HEAD section.
func (rq *Request) HeadHTML(w io.Writer) (err error) {
	if _, err = w.Write([]byte(rq.Jaws.headPrefix)); err == nil {
		if _, err = w.Write([]byte(rq.JawsKeyString())); err == nil {
			_, err = w.Write([]byte(`";</script><noscript><div class="jaws-alert">This site requires Javascript for full functionality.</div></noscript>`))
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
func (rq *Request) Session() *Session {
	return rq.session
}

// Get is shorthand for `Session().Get()` and returns the session value associated with the key, or nil.
// It no session is associated with the Request, returns nil.
func (rq *Request) Get(key string) interface{} {
	return rq.Session().Get(key)
}

// Set is shorthand for `Session().Set()` and sets a session value to be associated with the key.
// If value is nil, the key is removed from the session.
// Does nothing if there is no session is associated with the Request.
func (rq *Request) Set(key string, val interface{}) {
	rq.Session().Set(key, val)
}

// Context returns the Request's Context, which is derived from the
// WebSocket's HTTP requests Context.
func (rq *Request) Context() (ctx context.Context) {
	rq.mu.RLock()
	ctx = rq.ctx
	rq.mu.RUnlock()
	return
}

func (rq *Request) maintenance(deadline time.Time) bool {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	if !rq.running {
		if rq.ctx.Err() != nil {
			return true
		}
		if rq.Created.Before(deadline) {
			rq.cancelLocked(newErrNoWebSocketRequest(rq))
			return true
		}
	}
	return false
}

func (rq *Request) cancelLocked(err error) {
	if rq.JawsKey != 0 && rq.ctx.Err() == nil {
		if !rq.running {
			err = newErrPendingCancelled(rq, err)
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

func (rq *Request) TagsOf(elem *Element) (tags []interface{}) {
	if elem != nil {
		rq.mu.RLock()
		defer rq.mu.RUnlock()
		for tag, elems := range rq.tagMap {
			for _, e := range elems {
				if e == elem {
					tags = append(tags, tag)
					break
				}
			}
		}
	}
	return
}

// Register creates a new Element with the given tagitem as a valid target
// for dynamic updates.
//
// This function can accept a string or a Jid as the tagitem. A string
// will be interpreted as jaws.Tag(string).
//
// Returns a Jid, suitable for including as a HTML "id" attribute:
//
//	<div id="{{$.Register `footag`}}">
func (rq *Request) Register(tagitem interface{}, params ...interface{}) jid.Jid {
	switch data := tagitem.(type) {
	case jid.Jid:
		if elem := rq.getElementByJid(data); elem != nil {
			if _, ok := elem.Ui().(*UiHtml); ok {
				elem.ParseParams(params)
			}
		}
		return data
	case string:
		tagitem = Tag(data)
	}

	uib := &UiHtml{}
	elem := rq.NewElement(uib)
	uib.parseGetter(elem, tagitem)
	elem.ParseParams(params)
	return elem.jid
}

// Dirty marks all Elements that have one or more of the given tags as dirty.
func (rq *Request) Dirty(tags ...interface{}) {
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

func (rq *Request) NewElement(ui UI) *Element {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	return rq.newElementLocked(ui)
}

func (rq *Request) getElementByJidLocked(jid Jid) (elem *Element) {
	for _, e := range rq.elems {
		if e.jid == jid {
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

func (rq *Request) hasTagLocked(elem *Element, tag interface{}) bool {
	for _, e := range rq.tagMap[tag] {
		if elem == e {
			return true
		}
	}
	return false
}

func (rq *Request) HasTag(elem *Element, tag interface{}) (yes bool) {
	rq.mu.RLock()
	yes = rq.hasTagLocked(elem, tag)
	rq.mu.RUnlock()
	return
}

func (rq *Request) appendDirtyTags(tags []interface{}) {
	rq.mu.Lock()
	rq.todoDirt = append(rq.todoDirt, tags...)
	rq.mu.Unlock()
}

// Tag adds the given tags to the given Element.
func (rq *Request) tagExpanded(elem *Element, expandedtags []interface{}) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	for _, tag := range expandedtags {
		if !rq.hasTagLocked(elem, tag) {
			rq.tagMap[tag] = append(rq.tagMap[tag], elem)
		}
	}
}

// Tag adds the given tags to the given Element.
func (rq *Request) Tag(elem *Element, tags ...interface{}) {
	if elem != nil && len(tags) > 0 && elem.Request == rq {
		rq.tagExpanded(elem, MustTagExpand(elem.Request, tags))
	}
}

// GetElements returns a list of the UI elements in the Request that have the given tag(s).
func (rq *Request) GetElements(tagitem interface{}) (elems []*Element) {
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

// Done returns the Request completion channel.
func (rq *Request) Done() (ch <-chan struct{}) {
	rq.mu.RLock()
	ch = rq.ctx.Done()
	rq.mu.RUnlock()
	return
}

// process is the main message processing loop. Will unsubscribe broadcastMsgCh and close outboundMsgCh on exit.
func (rq *Request) process(broadcastMsgCh chan Message, incomingMsgCh <-chan wsMsg, outboundCh chan<- string) {
	jawsDoneCh := rq.Jaws.Done()
	ctxDoneCh := rq.Done()
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundCh))
	go rq.eventCaller(eventCallCh, outboundCh, eventDoneCh)

	defer func() {
		rq.Jaws.unsubscribe(broadcastMsgCh)
		rq.killSession()
		close(eventCallCh)
		for {
			select {
			case <-eventCallCh:
			case <-incomingMsgCh:
			case <-eventDoneCh:
				close(outboundCh)
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

	var wsQueue []wsMsg

	for {
		var tagmsg Message
		var wsmsg wsMsg
		var ok bool

		if len(wsQueue) > 0 {
			wsQueue = rq.sendQueue(outboundCh, wsQueue)
		}

		// empty the dirty tags list and call JawsUpdate()
		// for identified elements. this queues up wsMsg
		// in rq.wsQueue.
		for _, elem := range rq.makeUpdateList() {
			elem.Ui().JawsUpdate(elem)
		}

		// append pending WS messages to the queue
		// in the order of Element creation
		rq.mu.RLock()
		for _, elem := range rq.elems {
			wsQueue = append(wsQueue, elem.wsQueue...)
			elem.wsQueue = elem.wsQueue[:0]
		}
		rq.mu.RUnlock()

		if len(wsQueue) > 0 {
			wsQueue = rq.sendQueue(outboundCh, wsQueue)
		}

		select {
		case <-jawsDoneCh:
		case <-ctxDoneCh:
		case tagmsg, ok = <-broadcastMsgCh:
		case wsmsg, ok = <-incomingMsgCh:
			if ok {
				// incoming event message from the websocket
				if wsmsg.Jid.IsValid() {
					switch wsmsg.What {
					case what.Input, what.Click:
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
			wsQueue = append(wsQueue, wsMsg{
				Data: v + "\t" + strconv.Quote(tagmsg.Data),
				What: tagmsg.What,
				Jid:  -1,
			})
		default:
			todo = rq.GetElements(v)
		}

		switch tagmsg.What {
		case what.Reload, what.Redirect, what.Order, what.Alert:
			wsQueue = append(wsQueue, wsMsg{
				Jid:  0,
				Data: tagmsg.Data,
				What: tagmsg.What,
			})
		default:
			for _, elem := range todo {
				switch tagmsg.What {
				case what.Delete:
					wsQueue = append(wsQueue, wsMsg{
						Jid:  elem.jid,
						What: what.Delete,
					})
					rq.deleteElement(elem)
				case what.Input, what.Click:
					// Input or Click messages recieved here are from Request.Send() or broadcasts.
					// they won't be sent out on the WebSocket, but will queue up a
					// call to the event function (if any).
					// primary usecase is tests.
					rq.queueEvent(eventCallCh, eventFnCall{jid: elem.jid, wht: tagmsg.What, data: tagmsg.Data})
				case what.Hook:
					// "hook" messages are used to synchronously call an event function.
					// the function must not send any messages itself, but may return
					// an error to be sent out as an alert message.
					// primary usecase is tests.
					if err := rq.Jaws.Log(rq.callAllEventHandlers(elem.jid, tagmsg.What, tagmsg.Data)); err != nil {
						wsQueue = append(wsQueue, wsMsg{
							Data: tagmsg.Data,
							Jid:  elem.jid,
							What: what.Alert,
						})
					}
				case what.Update:
					elem.Ui().JawsUpdate(elem)
				default:
					wsQueue = append(wsQueue, wsMsg{
						Data: tagmsg.Data,
						Jid:  elem.jid,
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
					if e := rq.getElementByJidLocked(id); e != nil {
						elems = append(elems, e)
					}
				}
			}
		}
	} else {
		if e := rq.getElementByJidLocked(id); e != nil {
			elems = append(elems, e)
		}
	}
	rq.mu.RUnlock()

	for _, e := range elems {
		if err = callEventHandler(e.ui, e, wht, val); err != ErrEventUnhandled {
			return
		}
		for _, h := range e.handlers {
			if err = h.JawsEvent(e, wht, val); err != ErrEventUnhandled {
				return
			}
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

func (rq *Request) wsSend(outboundCh chan<- string, s string) {
	select {
	case <-rq.Done():
	case outboundCh <- s:
	default:
		panic(fmt.Errorf("jaws: %v: outbound message channel is full (%d) sending %s", rq, len(outboundCh), s))
	}
}

func (rq *Request) sendQueue(outboundCh chan<- string, wsQueue []wsMsg) []wsMsg {
	var sb strings.Builder
	for _, msg := range wsQueue {
		sb.WriteString(msg.Format())
	}
	rq.wsSend(outboundCh, sb.String())
	return wsQueue[:0]
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
	defer rq.mu.Unlock()
	for _, tag := range rq.todoDirt {
		for _, elem := range rq.tagMap[tag] {
			if !elem.updating {
				elem.updating = true
				todo = append(todo, elem)
			}
		}
	}
	for _, elem := range todo {
		elem.updating = false
	}
	rq.todoDirt = rq.todoDirt[:0]
	return
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundCh chan<- string, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		if err := rq.callAllEventHandlers(call.jid, call.wht, call.data); err != nil {
			var m wsMsg
			m.FillAlert(err)
			select {
			case outboundCh <- m.Format():
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

func (rq *Request) Writer(w io.Writer) RequestWriter {
	return RequestWriter{rq: rq, Writer: w}
}
