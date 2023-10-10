package jaws

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
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
	remoteIP  net.IP                  // (read-only) remote IP, or nil
	session   *Session                // (read-only) session, if established
	sendCh    chan Message            // (read-only) direct send message channel
	mu        deadlock.RWMutex        // protects following
	dirty     []interface{}           // dirty tags
	ctx       context.Context         // current context, derived from either Jaws or WS HTTP req
	cancelFn  context.CancelCauseFunc // cancel function
	connectFn ConnectFn               // a ConnectFn to call before starting message processing for the Request
	elems     []*Element
	tagMap    map[interface{}][]*Element
}

type eventFnCall struct {
	e    *Element
	wht  what.What
	data string
}

const maxWsQueueLengthPerElement = 20

var ErrWebsocketQueueOverflow = errors.New("websocket queue overflow")
var requestPool = sync.Pool{New: newRequest}

func newRequest() interface{} {
	rq := &Request{
		sendCh: make(chan Message),
		tagMap: make(map[interface{}][]*Element),
	}
	return rq
}

func getRequest(jw *Jaws, jawsKey uint64, hr *http.Request) (rq *Request) {
	rq = requestPool.Get().(*Request)
	rq.Jaws = jw
	rq.JawsKey = jawsKey
	rq.Initial = hr
	rq.Created = time.Now()
	rq.ctx, rq.cancelFn = context.WithCancelCause(context.Background())
	if hr != nil {
		rq.remoteIP = parseIP(hr.RemoteAddr)
		if sess := jw.getSessionLocked(getCookieSessionsIds(hr.Header, jw.CookieName), rq.remoteIP); sess != nil {
			sess.addRequest(rq)
			rq.session = sess
		}
	}
	return rq
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

func (rq *Request) start(hr *http.Request) (err error) {
	var actualIP net.IP
	ctx := context.Background()
	if hr != nil {
		actualIP = parseIP(hr.RemoteAddr)
		ctx = hr.Context()
	}
	rq.mu.Lock()
	if equalIP(rq.remoteIP, actualIP) {
		rq.ctx, rq.cancelFn = context.WithCancelCause(ctx)
	} else {
		err = fmt.Errorf("/jaws/%s: expected IP %q, got %q", rq.JawsKeyString(), rq.remoteIP.String(), actualIP.String())
	}
	rq.mu.Unlock()
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

func (rq *Request) recycle() {
	rq.Jaws.deactivate(rq)
	rq.mu.Lock()
	rq.Jaws = nil
	rq.JawsKey = 0
	rq.connectFn = nil
	rq.Initial = nil
	rq.Created = time.Time{}
	rq.ctx = context.Background()
	rq.cancelFn = nil
	rq.dirty = rq.dirty[:0]
	rq.remoteIP = nil
	rq.elems = rq.elems[:0]
	rq.killSessionLocked()
	// this gets optimized to calling the 'runtime.mapclear' function
	// we don't expect this to improve speed, but it will lower GC load
	for k := range rq.tagMap {
		delete(rq.tagMap, k)
	}
	rq.mu.Unlock()
	requestPool.Put(rq)
}

// HeadHTML returns the HTML code needed to write in the HTML page's HEAD section.
func (rq *Request) HeadHTML() template.HTML {
	s := rq.Jaws.headPrefix + rq.JawsKeyString() + `";</script>`
	return template.HTML(s) // #nosec G203
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

// Broadcast sends a broadcast to all Requests except the current one.
func (rq *Request) Broadcast(msg Message) {
	msg.from = rq
	rq.Jaws.Broadcast(msg)
}

// Context returns the Request's Context, which is derived from the
// WebSocket's HTTP requests Context.
func (rq *Request) Context() (ctx context.Context) {
	rq.mu.RLock()
	ctx = rq.ctx
	rq.mu.RUnlock()
	return
}

func (rq *Request) cancel(err error) {
	if rq != nil {
		rq.mu.RLock()
		cancelFn := rq.cancelFn
		rq.mu.RUnlock()
		cancelFn(err)
	}
}

func (rq *Request) getDoneCh() (jawsDoneCh, ctxDoneCh <-chan struct{}) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	if rq.Jaws == nil {
		panic("Request.Send(): request is dead")
	}
	jawsDoneCh = rq.Jaws.Done()
	ctxDoneCh = rq.ctx.Done()
	return
}

// Send a message to the current Request only.
// Returns true if the message was successfully sent.
func (rq *Request) Send(msg Message) bool {
	jawsDoneCh, ctxDoneCh := rq.getDoneCh()
	select {
	case <-jawsDoneCh:
	case <-ctxDoneCh:
	case rq.sendCh <- msg:
		return true
	}
	return false
}

// Alert attempts to show an alert message on the current request webpage if it has an HTML element with the id 'jaws-alert'.
// The lvl argument should be one of Bootstraps alert levels: primary, secondary, success, danger, warning, info, light or dark.
//
// The default JaWS javascript only supports Bootstrap.js dismissable alerts.
func (rq *Request) Alert(lvl, msg string) {
	rq.Send(Message{
		What: what.Alert,
		Data: lvl + "\n" + msg,
	})
}

// AlertError calls Alert if the given error is not nil.
func (rq *Request) AlertError(err error) {
	if err != nil {
		rq.Send(makeAlertDangerMessage(rq.Jaws.Log(err)))
	}
}

func (rq *Request) makeIdList(tags []interface{}) string {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	var b []byte
	seen := make(map[*Element]struct{})
	for _, tag := range tags {
		for _, elem := range rq.getElementsLocked(tag) {
			if _, ok := seen[elem]; !ok {
				seen[elem] = struct{}{}
				if len(b) > 0 {
					b = append(b, ' ')
				}
				b = elem.jid.Append(b)
			}
		}
	}
	return string(b)
}

// Redirect requests the current Request to navigate to the given URL.
func (rq *Request) Redirect(url string) {
	rq.Send(Message{
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
		if elem := rq.GetElement(data); elem != nil {
			if uib, ok := elem.Ui().(*UiHtml); ok {
				uib.parseParams(elem, params)
			}
		}
		return data
	case string:
		tagitem = Tag(data)
	}

	uib := &UiHtml{}
	elem := rq.NewElement(uib)
	uib.parseGetter(elem, tagitem)
	uib.parseParams(elem, params)
	rq.Dirty(uib.Tag)
	return elem.jid
}

// Dirty marks all Elements that have one or more of the given tags as dirty.
func (rq *Request) Dirty(tags ...interface{}) {
	rq.Jaws.setDirty(TagExpand(rq, tags, nil))
}

// wantMessage returns true if the Request want the message.
func (rq *Request) wantMessage(msg *Message) (yes bool) {
	if rq != nil && msg.from != rq {
		switch dest := msg.Dest.(type) {
		case string: // HTML id
			yes = true
		case *Element:
			yes = dest.Request == rq
		case jid.Jid:
			yes = rq.GetElement(dest) != nil
		default:
			rq.mu.RLock()
			_, yes = rq.tagMap[msg.Dest]
			rq.mu.RUnlock()
		}
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

func (rq *Request) GetElement(jid Jid) (e *Element) {
	if jid > 0 {
		rq.mu.RLock()
		for _, elem := range rq.elems {
			if elem.jid == jid {
				e = elem
				break
			}
		}
		rq.mu.RUnlock()
	}
	return
}

// GetElements returns a list of the UI elements in the Request that have the given tag.
func (rq *Request) getElementsLocked(tag interface{}) (elems []*Element) {
	if el, ok := rq.tagMap[tag]; ok {
		elems = append(elems, el...)
	}
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
	rq.dirty = append(rq.dirty, tags...)
	rq.mu.Unlock()
}

// Tag adds the given tags to the given Element.
func (rq *Request) Tag(elem *Element, tags ...interface{}) {
	if elem != nil && len(tags) > 0 && elem.Request == rq {
		var expandedtags []interface{}
		expandedtags = TagExpand(elem.Request, tags, expandedtags)
		rq.mu.Lock()
		defer rq.mu.Unlock()
		for _, tag := range expandedtags {
			if !rq.hasTagLocked(elem, tag) {
				rq.tagMap[tag] = append(rq.tagMap[tag], elem)
			}
		}
	}
}

// GetElements returns a list of the UI elements in the Request that have the given tag.
func (rq *Request) GetElements(tag interface{}) (elems []*Element) {
	rq.mu.RLock()
	elems = rq.getElementsLocked(tag)
	rq.mu.RUnlock()
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
		rq.killSession()
		rq.Jaws.deactivate(rq)
		rq.Jaws.unsubscribe(broadcastMsgCh)
		close(eventCallCh)
		for {
			select {
			case <-eventCallCh:
			case <-rq.sendCh:
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
		case tagmsg, ok = <-rq.sendCh:
		case tagmsg, ok = <-broadcastMsgCh:
		case wsmsg, ok = <-incomingMsgCh:
			if ok {
				// incoming event message from the websocket
				if elem := rq.GetElement(wsmsg.Jid); elem != nil {
					rq.queueEvent(eventCallCh, eventFnCall{e: elem, wht: wsmsg.What, data: wsmsg.Data})
				}
				continue
			}
		}

		if !ok {
			// one of the channels are closed, so we're done
			return
		}

		// prepare the data to send in the WS message
		var wsdata string
		switch data := tagmsg.Data.(type) {
		case nil:
			// do nothing
		case string:
			wsdata = data
		case template.HTML:
			wsdata = string(data)
		case []interface{}: // list of tags
			wsdata = rq.makeIdList(data)
		}

		// collect all elements marked with the tag in the message
		var todo []*Element
		switch v := tagmsg.Dest.(type) {
		case nil:
			// matches no elements
		case *Element:
			if v.Request == rq {
				todo = append(todo, v)
			}
		case Jid:
			if elem := rq.GetElement(v); elem != nil {
				todo = append(todo, elem)
			}
		case string:
			// target is a regular HTML ID
			wsQueue = append(wsQueue, wsMsg{
				Data: v + "\t" + wsdata,
				What: tagmsg.What,
				Jid:  -1,
			})
		default:
			rq.mu.RLock()
			todo = append(todo, rq.tagMap[tagmsg.Dest]...)
			rq.mu.RUnlock()
		}

		switch tagmsg.What {
		case what.Update:
			// do nothing, but used for triggering updates
		case what.Reload, what.Redirect, what.Order, what.Alert:
			wsQueue = append(wsQueue, wsMsg{
				Jid:  0,
				Data: wsdata,
				What: tagmsg.What,
			})
		default:
			for _, elem := range todo {
				switch tagmsg.What {
				case what.Delete:
					rq.deleteElement(elem)
				case what.Input, what.Click:
					// Input or Click messages recieved here are from Request.Send() or broadcasts.
					// they won't be sent out on the WebSocket, but will queue up a
					// call to the event function (if any).
					// primary usecase is tests.
					rq.queueEvent(eventCallCh, eventFnCall{e: elem, wht: tagmsg.What, data: wsdata})
				case what.Hook:
					// "hook" messages are used to synchronously call an event function.
					// the function must not send any messages itself, but may return
					// an error to be sent out as an alert message.
					// primary usecase is tests.
					if h, ok := elem.Ui().(EventHandler); ok {
						if errmsg := makeAlertDangerMessage(h.JawsEvent(elem, tagmsg.What, wsdata)); errmsg.What != what.Update {
							wsQueue = append(wsQueue, wsMsg{
								Data: wsdata,
								Jid:  elem.jid,
								What: errmsg.What,
							})
						}
					}
				default:
					wsQueue = append(wsQueue, wsMsg{
						Data: wsdata,
						Jid:  elem.jid,
						What: tagmsg.What,
					})
				}
			}
		}
	}
}

func (rq *Request) queueEvent(eventCallCh chan eventFnCall, call eventFnCall) {
	select {
	case eventCallCh <- call:
	default:
		rq.Jaws.MustLog(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, call))
		return
	}
}

func (rq *Request) send(outboundCh chan<- string, s string) {
	select {
	case <-rq.Jaws.Done():
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
	rq.send(outboundCh, sb.String())
	return wsQueue[:0]
}

func (rq *Request) deleteElement(e *Element) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	e.Request = nil
	rq.elems = slices.DeleteFunc(rq.elems, func(elem *Element) bool { return elem == e })
	for k := range rq.tagMap {
		rq.tagMap[k] = slices.DeleteFunc(rq.tagMap[k], func(elem *Element) bool { return elem == e })
	}
}

func (rq *Request) makeUpdateList() (todo []*Element) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	for _, tag := range rq.dirty {
		if elem, ok := tag.(*Element); ok {
			if !elem.updating {
				elem.updating = true
				todo = append(todo, elem)
			}
			continue
		}
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
	rq.dirty = rq.dirty[:0]
	return
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundCh chan<- string, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		var err error
		if h, ok := call.e.Ui().(EventHandler); ok {
			err = h.JawsEvent(call.e, call.wht, call.data)
		}
		if err != nil {
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

func makeAlertDangerMessage(err error) (msg Message) {
	if err != nil {
		msg = Message{
			Data: "danger\n" + html.EscapeString(err.Error()),
			What: what.Alert,
		}
	}
	return
}
