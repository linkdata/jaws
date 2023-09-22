package jaws

import (
	"fmt"
	"html"
	"html/template"
	"log"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

// ConnectFn can be used to interact with a Request before message processing starts.
// Returning an error causes the Request to abort, and the WebSocket connection to close.
type ConnectFn = func(rq *Request) error

// EventFn is the signature of a event handling function to be called when JaWS receives
// an event message from the Javascript via the WebSocket connection.
type EventFn = func(rq *Request, wht what.What, id, val string) error

// Request maintains the state for a JaWS WebSocket connection, and handles processing
// of events and broadcasts.
//
// Note that we have to store the context inside the struct because there is no call chain
// between the Request being created and it being used once the WebSocket is created.
type Request struct {
	Jaws      *Jaws            // (read-only) the JaWS instance the Request belongs to
	JawsKey   uint64           // (read-only) a random number used in the WebSocket URI to identify this Request
	Created   time.Time        // (read-only) when the Request was created, used for automatic cleanup
	Initial   *http.Request    // (read-only) initial HTTP request passed to Jaws.NewRequest
	remoteIP  net.IP           // (read-only) remote IP, or nil
	session   *Session         // (read-only) session, if established
	sendCh    chan Message     // (read-only) direct send message channel
	mu        deadlock.RWMutex // protects following
	dirty     []interface{}    // dirty tags
	wsreq     *http.Request    // (read-only) WebSocket HTTP request passed to Jaws.UseRequest
	connectFn ConnectFn        // a ConnectFn to call before starting message processing for the Request
	elems     []*Element
	tagMap    map[interface{}][]*Element
}

type eventFnCall struct {
	e    *Element
	wht  what.What
	data string
}

func (call *eventFnCall) String() string {
	return fmt.Sprintf("eventFnCall{%v, %s, %q}", call.e, call.wht, call.data)
}

var requestPool = sync.Pool{New: func() interface{} {
	return &Request{
		sendCh: make(chan Message),
		tagMap: make(map[interface{}][]*Element),
	}
}}

func newRequest(jw *Jaws, jawsKey uint64, hr *http.Request) (rq *Request) {
	rq = requestPool.Get().(*Request)
	rq.Jaws = jw
	rq.JawsKey = jawsKey
	rq.Created = time.Now()
	rq.Initial = hr
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
	if hr != nil {
		actualIP = parseIP(hr.RemoteAddr)
	}
	rq.mu.Lock()
	if equalIP(rq.remoteIP, actualIP) {
		rq.wsreq = hr
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
	rq.wsreq = nil
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

func (rq *Request) getDoneCh() (jawsDoneCh, ctxDoneCh <-chan struct{}) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	if rq.Jaws == nil {
		panic("Request.Send(): request is dead")
	}
	jawsDoneCh = rq.Jaws.Done()
	if rq.wsreq != nil {
		ctxDoneCh = rq.wsreq.Context().Done()
	}
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

func (rq *Request) makeOrder(tags []interface{}) string {
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
				b = elem.jid.AppendInt(b)
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

// Register creates a new Element with the given tag as a valid target
// for dynamic updates.
//
// Returns the JaWS ID, suitable for including as a HTML attribute:
//
//	<div id="{{$.Register `footag`}}">
func (rq *Request) Register(tagitem interface{}, params ...interface{}) Jid {
	switch data := tagitem.(type) {
	case Jid:
		if elem := rq.GetElement(data); elem != nil {
			if uib, ok := elem.UI().(*UiHtml); ok {
				uib.parseParams(elem, params)
			}
			return data
		}
		return 0
	case string:
		tagitem = Tag{data}
	case template.HTML:
		tagitem = Tag{string(data)}
	}

	for _, elem := range rq.GetElements(tagitem) {
		if uib, ok := elem.UI().(*UiHtml); ok {
			uib.parseParams(elem, params)
		}
	}

	uib := &UiHtml{}
	elem := rq.NewElement(uib)
	elem.Tag(tagitem)
	uib.parseParams(elem, params)
	return elem.jid
}

// wantMessage returns true if the Request want the message.
func (rq *Request) wantMessage(msg *Message) (yes bool) {
	if rq != nil && msg.from != rq {
		rq.mu.RLock()
		_, yes = rq.tagMap[msg.Tag]
		rq.mu.RUnlock()
	}
	return
}

func (rq *Request) newElementLocked(ui UI) (elem *Element) {
	elem = &Element{
		jid:     Jid(len(rq.elems) + 1),
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
		if int(jid) <= len(rq.elems) {
			e = rq.elems[jid-1]
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

// Dirty marks all Elements with the given tags as dirty.
func (rq *Request) Dirty(tags ...interface{}) {
	rq.mu.Lock()
	rq.dirty = append(rq.dirty, tags...)
	rq.mu.Unlock()
}

// Tag adds the given tags to the given Element.
func (rq *Request) Tag(elem *Element, tags ...interface{}) {
	if elem != nil {
		var expandedtags []interface{}
		expandedtags = TagExpand(tags, expandedtags)
		rq.mu.Lock()
		defer rq.mu.Unlock()
		for _, e := range rq.elems {
			if e == elem {
				for _, tag := range expandedtags {
					if !rq.hasTagLocked(elem, tag) {
						rq.tagMap[tag] = append(rq.tagMap[tag], elem)
					}
				}
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
	if rq.wsreq != nil {
		ch = rq.wsreq.Context().Done()
	}
	rq.mu.RUnlock()
	return
}

// process is the main message processing loop. Will unsubscribe broadcastMsgCh and close outboundMsgCh on exit.
func (rq *Request) process(broadcastMsgCh chan Message, incomingMsgCh <-chan wsMsg, outboundMsgCh chan<- wsMsg) {
	if deadlock.Debug {
		rq.Jaws.mu.RLock()
		_, ok := rq.Jaws.active[rq]
		rq.Jaws.mu.RUnlock()
		if !ok {
			log.Panicf("Request %v is not in active map\n", rq)
		}
	}
	jawsDoneCh := rq.Jaws.Done()
	ctxDoneCh := rq.Done()
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundMsgCh))
	go rq.eventCaller(eventCallCh, outboundMsgCh, eventDoneCh)

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

	var dirtyTags []interface{}

	for {
		var tagmsg Message
		var wsmsg wsMsg
		var ok bool

		select {
		case <-jawsDoneCh:
		case <-ctxDoneCh:
		case tagmsg, ok = <-rq.sendCh:
		case tagmsg, ok = <-broadcastMsgCh:
		case wsmsg, ok = <-incomingMsgCh:
			if ok {
				// incoming event message from the websocket
				if elem := rq.GetElement(wsmsg.Jid); elem != nil {
					select {
					case eventCallCh <- eventFnCall{e: elem, wht: wsmsg.What, data: wsmsg.Data}:
					default:
						rq.Jaws.MustLog(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, tagmsg))
						return
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
		if tagmsg.What == what.Remove {
			rq.mu.Lock()
			todo = append(todo, rq.tagMap[tagmsg.Tag]...)
			for _, elem := range todo {
				if elem != nil {
					rq.elems[elem.jid-1] = nil
				}
			}
			delete(rq.tagMap, tagmsg.Tag)
			rq.mu.Unlock()
		} else {
			rq.mu.RLock()
			todo = append(todo, rq.tagMap[tagmsg.Tag]...)
			rq.mu.RUnlock()
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
			wsdata = rq.makeOrder(data)
		}

		switch tagmsg.What {
		case what.None:
			// do nothing
		case what.Update:
			// do nothing
		case what.Reload:
			fallthrough
		case what.Redirect:
			fallthrough
		case what.Order:
			fallthrough
		case what.Alert:
			rq.send(outboundMsgCh, wsMsg{
				Data: wsdata,
				What: tagmsg.What,
			})
		default:
			for _, elem := range todo {
				switch tagmsg.What {
				case what.Trigger:
					// trigger messages won't be sent out on the WebSocket, but will queue up a
					// call to the event function (if any)
					select {
					case eventCallCh <- eventFnCall{e: elem, wht: tagmsg.What, data: wsdata}:
					default:
						rq.Jaws.MustLog(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, tagmsg))
						return
					}
				case what.Hook:
					// "hook" messages are used to synchronously call an event function.
					// the function must not send any messages itself, but may return
					// an error to be sent out as an alert message.
					// primary usecase is tests.
					if h, ok := elem.UI().(EventHandler); ok {
						if errmsg := makeAlertDangerMessage(h.JawsEvent(elem, tagmsg.What, wsdata)); errmsg.What != what.None {
							rq.send(outboundMsgCh, wsMsg{
								Jid:  elem.jid,
								What: errmsg.What,
								Data: wsdata,
							})
						}
					}
				default:
					rq.send(outboundMsgCh, wsMsg{
						Jid:  elem.jid,
						What: tagmsg.What,
						Data: wsdata,
					})
				}
			}
		}

		rq.mu.Lock()
		dirtyTags, rq.dirty = rq.dirty, dirtyTags
		rq.dirty = rq.dirty[:0]
		rq.mu.Unlock()

		if len(dirtyTags) > 0 {
			rq.callUpdate(outboundMsgCh, dirtyTags)
		}
	}
}

func (rq *Request) send(outboundMsgCh chan<- wsMsg, msg wsMsg) {
	select {
	case <-rq.Jaws.Done():
	case <-rq.Done():
	case outboundMsgCh <- msg:
	default:
		panic(fmt.Errorf("jaws: %v: outbound message channel is full sending %s", rq, msg))
	}
}

func (rq *Request) callUpdate(outboundMsgCh chan<- wsMsg, dirtyTags []interface{}) {
	order := 0
	m := make(map[*Element]int, len(rq.elems))
	rq.mu.RLock()
	for _, tag := range dirtyTags {
		order++
		if elem, ok := tag.(*Element); ok {
			m[elem] = order
		} else {
			for _, elem := range rq.tagMap[tag] {
				m[elem] = order
			}
		}
	}
	rq.mu.RUnlock()

	var todo []Updater
	for elem, order := range m {
		if elem != nil {
			todo = append(todo, Updater{outCh: outboundMsgCh, order: order, Element: elem})
		}
	}
	sort.Slice(todo, func(i, j int) bool {
		return todo[i].order < todo[j].order
	})
	for _, u := range todo {
		u.UI().JawsUpdate(u)
	}
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundMsgCh chan<- wsMsg, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		var err error
		switch call.wht {
		case what.Click:
			if h, ok := call.e.UI().(ClickHandler); ok {
				err = h.JawsClick(call.e, call.data)
				break
			}
			fallthrough
		case what.Input, what.Trigger:
			if h, ok := call.e.UI().(EventHandler); ok {
				err = h.JawsEvent(call.e, call.wht, call.data)
				break
			}
			fallthrough
		default:
			if deadlock.Debug {
				err = rq.Jaws.Log(fmt.Errorf("jaws: eventCaller unhandled: %s", call.String()))
			}
		}
		if err != nil {
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

func makeAlertDangerMessage(err error) (msg Message) {
	if err != nil {
		msg = Message{
			Data: "danger\n" + html.EscapeString(err.Error()),
			What: what.Alert,
		}
	}
	return
}

// OnTrigger registers a jid and a function to be called when Trigger is called for it.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnTrigger(jid string, fn func(rq *Request, jid string) error) error {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, evt what.What, jid, val string) (err error) {
			if evt == what.Trigger {
				err = fn(rq, jid)
			}
			return
		}
	}
	rq.Register(Tag{jid}, wf)
	return nil
}
