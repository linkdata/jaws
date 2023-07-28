package jaws

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"strings"
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
	Context   context.Context  // (read-only) context passed to Jaws.NewRequest
	remoteIP  net.IP           // (read-only) remote IP, or nil
	session   *Session         // (read-only) session, if established
	sendCh    chan *Message    // (read-only) direct send message channel
	mu        deadlock.RWMutex // protects following
	connectFn ConnectFn        // a ConnectFn to call before starting message processing for the Request
	nextJid   int
	elems     []*Element
	tagMap    map[interface{}][]*Element
}

type eventFnCall struct {
	e    *Element
	wht  what.What
	data string
}

var metaIds = map[interface{}]struct{}{
	" reload":   {},
	" ping":     {},
	" redirect": {},
	" alert":    {},
}

var requestPool = sync.Pool{New: func() interface{} {
	return &Request{
		tagMap: make(map[interface{}][]*Element),
		sendCh: make(chan *Message),
	}
}}

func newRequest(ctx context.Context, jw *Jaws, jawsKey uint64, hr *http.Request) (rq *Request) {
	rq = requestPool.Get().(*Request)
	rq.Jaws = jw
	rq.JawsKey = jawsKey
	rq.Created = time.Now()
	rq.Initial = hr
	rq.Context = ctx
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

func (rq *Request) start(hr *http.Request) error {
	rq.mu.RLock()
	expectIP := rq.remoteIP
	rq.mu.RUnlock()
	var actualIP net.IP
	if hr != nil {
		actualIP = parseIP(hr.RemoteAddr)
	}
	if equalIP(expectIP, actualIP) {
		return nil
	}
	return fmt.Errorf("/jaws/%s: expected IP %q, got %q", rq.JawsKeyString(), expectIP.String(), actualIP.String())
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
	rq.mu.Lock()
	rq.Jaws = nil
	rq.JawsKey = 0
	rq.connectFn = nil
	rq.Initial = nil
	rq.Context = nil
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
func (rq *Request) Broadcast(msg *Message) {
	msg.from = rq
	rq.Jaws.Broadcast(msg)
}

// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests except this one.
func (rq *Request) Trigger(id, val string) {
	rq.Broadcast(&Message{
		Tag:  id,
		What: what.Trigger,
		Data: val,
	})
}

// SetInner sends a jid and new inner HTML to all Requests except this one.
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetInner(jid string, innerHtml string) {
	rq.Broadcast(&Message{
		Tag:  jid,
		What: what.Inner,
		Data: innerHtml,
	})
}

// SetTextValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetTextValue(jid, val string) {
	rq.Broadcast(&Message{
		Tag:  jid,
		What: what.Value,
		Data: val,
	})
}

// SetFloatValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetFloatValue(jid string, val float64) {
	rq.Broadcast(&Message{
		Tag:  jid,
		What: what.Value,
		Data: strconv.FormatFloat(val, 'f', -1, 64),
	})
}

// SetBoolValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetBoolValue(jid string, val bool) {
	rq.Broadcast(&Message{
		Tag:  jid,
		What: what.Value,
		Data: strconv.FormatBool(val),
	})
}

// SetDateValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetDateValue(jid string, val time.Time) {
	rq.Broadcast(&Message{
		Tag:  jid,
		What: what.Value,
		Data: val.Format(ISO8601),
	})
}

func (rq *Request) getDoneCh(msg *Message) (<-chan struct{}, <-chan struct{}) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	if rq.Jaws == nil {
		panic(fmt.Sprintf("Request.Send(%v): request is dead", msg))
	}
	return rq.Jaws.Done(), rq.Context.Done()
}

// Send a message to the current Request only.
// Returns true if the message was successfully sent.
func (rq *Request) Send(msg *Message) bool {
	jawsDoneCh, ctxDoneCh := rq.getDoneCh(msg)
	select {
	case <-jawsDoneCh:
	case <-ctxDoneCh:
	case rq.sendCh <- msg:
		return true
	}
	return false
}

// SetAttr sets an attribute on the HTML element(s) on the current Request only.
// If the value is an empty string, a value-less attribute will be added (such as "disabled").
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetAttr(jid, attr, val string) {
	rq.Send(&Message{
		Tag:  jid,
		What: what.SAttr,
		Data: attr + "\n" + val,
	})
}

// RemoveAttr removes a given attribute from the HTML element(s) for the current Request only.
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (rq *Request) RemoveAttr(jid, attr string) {
	rq.Send(&Message{
		Tag:  jid,
		What: what.RAttr,
		Data: attr,
	})
}

// Alert attempts to show an alert message on the current request webpage if it has an HTML element with the id 'jaws-alert'.
// The lvl argument should be one of Bootstraps alert levels: primary, secondary, success, danger, warning, info, light or dark.
//
// The default JaWS javascript only supports Bootstrap.js dismissable alerts.
func (rq *Request) Alert(lvl, msg string) {
	rq.Send(&Message{
		Tag:  " alert",
		Data: lvl + "\n" + msg,
	})
}

// AlertError calls Alert if the given error is not nil.
func (rq *Request) AlertError(err error) {
	if err != nil {
		rq.Send(makeAlertDangerMessage(rq.Jaws.Log(err)))
	}
}

// Redirect requests the current Request to navigate to the given URL.
func (rq *Request) Redirect(url string) {
	rq.Send(&Message{
		Tag:  " redirect",
		Data: url,
	})
}

// RegisterEventFn records the given tag string as a valid target
// for dynamic updates using the given event function (which may be nil).
//
// If the tagstring argument is empty, a unique tag will be generated.
// The tagstring may not contains spaces.
//
// If fn argument is nil, a pre-existing event function won't be overwritten.
//
// Returns the (possibly generated) tagstring.
func (rq *Request) RegisterEventFn(tagstring string, fn EventFn) string {
	if strings.ContainsRune(tagstring, ' ') {
		panic("jaws: RegisterEventFn: tagstring contains spaces")
	}
	if tagstring == "" {
		tagstring = MakeID()
	}
	tags := []interface{}{tagstring}
	rq.mu.Lock()
	defer rq.mu.Unlock()

	var missing []interface{}
	for _, tag := range tags {
		if elems, ok := rq.tagMap[tag]; ok {
			if fn != nil {
				for _, elem := range elems {
					if uib, ok := elem.Ui.(*UiHtml); ok {
						uib.EventFn = fn
					}
				}
			}
		} else {
			missing = append(missing, tag)
		}
	}
	rq.newElementLocked(missing, &UiHtml{Tags: tags, EventFn: fn}, nil)

	return tagstring
}

// Register calls RegisterEventFn(tagstring, nil).
// Useful in template constructs like:
//
//	<div jid="{{$.Register `foo`}}">
func (rq *Request) Register(tagstring string) string {
	return rq.RegisterEventFn(tagstring, nil)
}

// HasTag returns true if the Request has one or more UI elements that have the given tag.
func (rq *Request) HasTag(tag interface{}) (ok bool) {
	rq.mu.RLock()
	_, ok = rq.tagMap[tag]
	rq.mu.RUnlock()
	return
}

// GetElements returns a list of the UI elements in the Request that have the given tag.
func (rq *Request) GetElements(tag interface{}) (elems []*Element) {
	rq.mu.RLock()
	if el, ok := rq.tagMap[tag]; ok {
		elems = append(elems, el...)
	}
	rq.mu.RUnlock()
	return
}

// process is the main message processing loop. Will unsubscribe broadcastMsgCh and close outboundMsgCh on exit.
func (rq *Request) process(broadcastMsgCh chan *Message, incomingMsgCh <-chan wsMsg, outboundMsgCh chan<- wsMsg) {
	jawsDoneCh := rq.Jaws.Done()
	ctxDoneCh := rq.Context.Done()
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundMsgCh))
	go rq.eventCaller(eventCallCh, outboundMsgCh, eventDoneCh)

	defer func() {
		rq.killSession()
		rq.Jaws.unsubscribe(broadcastMsgCh)
		close(eventCallCh)
		for {
			select {
			case <-eventCallCh:
			case <-rq.sendCh:
			case <-incomingMsgCh:
			case <-eventDoneCh:
				close(outboundMsgCh)
				return
			}
		}
	}()

	for {
		var tagmsg *Message

		select {
		case <-jawsDoneCh:
			return
		case <-ctxDoneCh:
			return
		case tagmsg = <-rq.sendCh:
		case tagmsg = <-broadcastMsgCh:
		case wsmsg, ok := <-incomingMsgCh:
			if !ok {
				return
			}
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

		if tagmsg == nil {
			// one of the channels are closed, so we're done
			return
		}

		var todo []*Element
		todo = append(todo, rq.tagMap[tagmsg.Tag]...)
		if _, ok := metaIds[tagmsg.Tag]; ok {
			todo = append(todo, &Element{
				Ui:      nil,
				jid:     tagmsg.Tag.(string),
				Request: rq,
			})
		}

		// find all elements listening to one of the tags in the message
		for _, elem := range todo {
			// messages incoming from WebSocket or trigger messages
			// won't be sent out on the WebSocket, but will queue up a
			// call to the event function (if any)
			if tagmsg.What == what.Trigger {
				select {
				case eventCallCh <- eventFnCall{e: elem, wht: tagmsg.What, data: tagmsg.Data}:
				default:
					rq.Jaws.MustLog(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, tagmsg))
					return
				}
				continue
			}

			// "hook" messages are used to synchronously call an event function.
			// the function must not send any messages itself, but may return
			// an error to be sent out as an alert message.
			// primary usecase is tests.
			if tagmsg.What == what.Hook {
				tagmsg = makeAlertDangerMessage(elem.Ui.JawsEvent(elem, tagmsg.What, tagmsg.Data))
			}

			if tagmsg != nil {
				select {
				case <-jawsDoneCh:
				case <-ctxDoneCh:
				case outboundMsgCh <- wsMsg{
					Jid:  elem.Jid(),
					What: tagmsg.What,
					Data: tagmsg.Data,
				}:
				default:
					rq.Jaws.MustLog(fmt.Errorf("jaws: %v: outboundMsgCh is full sending %v", rq, tagmsg))
					return
				}
			}
		}
	}
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundMsgCh chan<- wsMsg, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		if err := call.e.Ui.JawsEvent(call.e, call.wht, call.data); err != nil {
			select {
			case outboundMsgCh <- wsMsg{
				Jid:  " alert",
				Data: "danger\n" + html.EscapeString(err.Error()),
			}:
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

func makeAlertDangerMessage(err error) (msg *Message) {
	if err != nil {
		msg = &Message{
			Tag:  " alert",
			Data: "danger\n" + html.EscapeString(err.Error()),
		}
	}
	return
}

func (rq *Request) maybeEvent(event what.What, jid string, fn func(rq *Request, jid string) error) string {
	var wf EventFn
	if fn != nil {
		wf = func(rq *Request, evt what.What, jid, val string) (err error) {
			if evt == event {
				err = fn(rq, jid)
			}
			return
		}
	}
	return rq.RegisterEventFn(jid, wf)
}
