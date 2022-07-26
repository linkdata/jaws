package jaws

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net"
	"strconv"
	"sync"
	"time"
)

// ConnectFn can be used to interact with a Request before message processing starts.
// Returning an error causes the Request to abort, and the WebSocket connection to close.
type ConnectFn func(rq *Request) error

// EventFn is the signature of a event handling function to be called when JaWS receives
// an event message from the Javascript via the WebSocket connection.
type EventFn func(rq *Request, id, evt, val string) error

// Request maintains the state for a JaWS WebSocket connection, and handles processing
// of events and broadcasts.
//
// Note that we have to store the context inside the struct because there is no call chain
// between the Request being created and it being used once the WebSocket is created.
type Request struct {
	Jaws      *Jaws              // the JaWS instance the Request belongs to
	JawsKey   int64              // a random number used in the WebSocket URI to identify this Request
	ConnectFn ConnectFn          // a ConnectFn to call before starting message processing for the Request
	Started   time.Time          // when the Request was started, used for automatic cleanup
	ctx       context.Context    // context passed to NewRequest
	remoteIP  net.IP             // parsed remote IP (or nil)
	sendCh    chan *Message      // direct send message channel
	mu        sync.RWMutex       // protects following
	elems     map[string]EventFn // map of registered HTML id's
}

type eventFnCall struct {
	fn  EventFn
	msg *Message
}

var metaIds = map[string]struct{}{
	" reload":   {},
	" redirect": {},
	" alert":    {},
}

var requestPool = sync.Pool{New: func() interface{} {
	return &Request{
		elems:  make(map[string]EventFn),
		sendCh: make(chan *Message),
	}
}}

func newRequest(ctx context.Context, j *Jaws, key int64, remoteAddr string) (rq *Request) {
	rq = requestPool.Get().(*Request)
	rq.Jaws = j
	rq.JawsKey = key
	rq.Started = time.Now()
	rq.ctx = ctx
	rq.remoteIP = parseIP(remoteAddr)
	return rq
}

func (rq *Request) JawsKeyString() string {
	jawsKey := int64(-1)
	if rq != nil {
		jawsKey = rq.JawsKey
	}
	return JawsKeyString(jawsKey)
}

func (rq *Request) String() string {
	return "Request<" + rq.JawsKeyString() + ">"
}

func (rq *Request) recycle() {
	rq.mu.Lock()
	rq.Jaws = nil
	rq.JawsKey = -1
	rq.ConnectFn = nil
	rq.ctx = nil
	rq.remoteIP = nil
	// this gets optimized to calling the 'runtime.mapclear' function
	// we don't expect this to improve speed, but it will lower GC load
	for k := range rq.elems {
		delete(rq.elems, k)
	}
	rq.mu.Unlock()
	requestPool.Put(rq)
}

// HeadHTML returns the HTML code needed to write in the HTML page's HEAD section.
func (rq *Request) HeadHTML() template.HTML {
	return HeadHTML(rq.JawsKey, nil)
}

// Context returns the context passed to NewRequest()
func (rq *Request) Context() (ctx context.Context) {
	rq.mu.RLock()
	ctx = rq.ctx
	rq.mu.RUnlock()
	return
}

// Broadcast sends a broadcast to all Requests except the current one.
func (rq *Request) Broadcast(msg *Message) {
	msg.from = rq
	rq.Jaws.Broadcast(msg)
}

// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests except this one.
func (rq *Request) Trigger(id, val string) {
	rq.Broadcast(&Message{
		Elem: id,
		What: "trigger",
		Data: val,
	})
}

// SetInner sends an HTML id and new inner HTML to all Requests except this one.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetInner(id string, innerHtml string) {
	rq.Broadcast(&Message{
		Elem: id,
		What: "inner",
		Data: innerHtml,
	})
}

// SetTextValue sends an HTML id and new input value to all Requests except this one.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetTextValue(id, val string) {
	rq.Broadcast(&Message{
		Elem: id,
		What: "value",
		Data: val,
	})
}

// SetFloatValue sends an HTML id and new input value to all Requests except this one.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetFloatValue(id string, val float64) {
	rq.Broadcast(&Message{
		Elem: id,
		What: "value",
		Data: strconv.FormatFloat(val, 'f', -1, 64),
	})
}

// SetBoolValue sends an HTML id and new input value to all Requests except this one.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetBoolValue(id string, val bool) {
	rq.Broadcast(&Message{
		Elem: id,
		What: "value",
		Data: strconv.FormatBool(val),
	})
}

// SetDateValue sends an HTML id and new input value to all Requests except this one.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetDateValue(id string, val time.Time) {
	rq.Broadcast(&Message{
		Elem: id,
		What: "value",
		Data: val.Format(ISO8601),
	})
}

// Send queues up a message for sending to the current Request only.
// Returns true if the message was successfully queued for sending.
func (rq *Request) Send(msg *Message) bool {
	select {
	case <-rq.Jaws.Done():
	case <-rq.Context().Done():
	case rq.sendCh <- msg:
		return true
	}
	return false
}

// SetAttr sets an attribute on the HTML element on the current Request only.
// If the value is an empty string, a value-less attribute will be added (such as "disabled").
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetAttr(id, attr, val string) {
	rq.Send(&Message{
		Elem: id,
		What: "sattr",
		Data: attr + "\n" + val,
	})
}

// RemoveAttr removes a given attribute from the HTML id for the current Request only.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (rq *Request) RemoveAttr(id, attr string) {
	rq.Send(&Message{
		Elem: id,
		What: "rattr",
		Data: attr,
	})
}

// Alert attempts to show an alert message on the current request webpage if it has an HTML element with the id 'jaws-alert'.
//
// The default JaWS javascript only supports Bootstrap.js dismissable alerts.
func (rq *Request) Alert(lvl, msg string) {
	rq.Send(&Message{
		Elem: " alert",
		What: lvl,
		Data: msg,
	})
}

// AlertError calls Alert if the given error is not nil.
func (rq *Request) AlertError(err error) {
	if err != nil {
		rq.Jaws.Log(err)
		rq.Send(makeAlertDangerMessage(err))
	}
}

// Redirect requests the current Request to navigate to the given URL.
func (rq *Request) Redirect(url string) {
	rq.Send(&Message{
		Elem: " redirect",
		What: url,
	})
}

// RegisterEventFn records the given HTML element ID as a valid target for dynamic updates
// using the given event function (which may be nil).
// If the id argument is the empty or blank, a unique ID will be generated.
// If fn is nil, the existing event handler function handler won't be overwritten.
// Returns the (possibly generated) id.
func (rq *Request) RegisterEventFn(id string, fn EventFn) string {
	if id == "" {
		id = rq.Jaws.MakeID()
	}
	rq.mu.Lock()
	if fn == nil {
		if _, ok := rq.elems[id]; !ok {
			rq.elems[id] = nil
		}
	} else {
		rq.elems[id] = fn
	}
	rq.mu.Unlock()
	return id
}

// Register calls RegisterEventFn(id, nil).
// Useful in template constructs like:
//   <div id="{{$.Register `foo`}}">
func (rq *Request) Register(id string) string {
	return rq.RegisterEventFn(id, nil)
}

// GetEventFn checks if a given HTML element is registered and returns
// the it's event function (or nil) along with a boolean indicating
// if it's a registered HTML id.
func (rq *Request) GetEventFn(id string) (fn EventFn, ok bool) {
	rq.mu.RLock()
	if fn, ok = rq.elems[id]; !ok {
		_, ok = metaIds[id]
	}
	rq.mu.RUnlock()
	return
}

// SetEventFn sets the event function for the given HTML ID to be the given function.
// Passing nil for the function is legal, and has the effect of ensuring the
// ID can be the target of DOM updates but not to send Javascript events.
// Note that you can only have one event function per ID.
func (rq *Request) SetEventFn(id string, fn EventFn) {
	rq.mu.Lock()
	rq.elems[id] = fn
	rq.mu.Unlock()
}

// OnEvent calls SetEventFn.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnEvent(id string, fn EventFn) error {
	rq.SetEventFn(id, fn)
	return nil
}

// process is the main message processing loop. Will unsubscribe broadcastMsgCh and close outboundMsgCh on exit.
func (rq *Request) process(broadcastMsgCh chan *Message, incomingMsgCh <-chan *Message, outboundMsgCh chan<- *Message) {
	jawsDoneCh := rq.Jaws.Done()
	ctxDoneCh := rq.Context().Done()
	eventDoneCh := make(chan struct{})
	eventCallCh := make(chan eventFnCall, cap(outboundMsgCh))
	go rq.eventCaller(eventCallCh, outboundMsgCh, eventDoneCh)

	defer func() {
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
		var msg *Message
		incoming := false

		select {
		case <-jawsDoneCh:
		case <-ctxDoneCh:
		case msg = <-rq.sendCh:
		case msg = <-broadcastMsgCh:
		case msg = <-incomingMsgCh:
			// messages incoming from the WebSocket are not to be resent out on
			// the WebSocket again, so note that this is an incoming message
			incoming = true
		}

		if msg == nil {
			// one of the channels are closed, so we're done
			return
		}

		if msg.from == rq {
			// don't process broadcasts that originate from ourselves
			continue
		}

		// only ever process messages for registered elements
		if fn, ok := rq.GetEventFn(msg.Elem); ok {
			// messages incoming from WebSocket or trigger messages
			// won't be sent out on the WebSocket, but will queue up a
			// call to the event function (if any)
			if incoming || msg.What == "trigger" {
				if fn != nil {
					select {
					case eventCallCh <- eventFnCall{fn: fn, msg: msg}:
					default:
						rq.Jaws.MustLog(fmt.Errorf("jaws: %v: eventCallCh is full sending %v", rq, msg))
						return
					}
				}
				continue
			}

			// "hook" messages are used to synchronously call an event function.
			// the function must not send any messages itself, but may return
			// an error to be sent out as an alert message.
			// primary usecase is tests.
			if msg.What == "hook" {
				msg = makeAlertDangerMessage(fn(rq, msg.Elem, msg.What, msg.Data))
			}

			if msg != nil {
				select {
				case <-jawsDoneCh:
				case <-ctxDoneCh:
				case outboundMsgCh <- msg:
				default:
					rq.Jaws.MustLog(fmt.Errorf("jaws: %v: outboundMsgCh is full sending %v", rq, msg))
					return
				}
			}
		}
	}
}

// eventCaller calls event functions
func (rq *Request) eventCaller(eventCallCh <-chan eventFnCall, outboundMsgCh chan<- *Message, eventDoneCh chan<- struct{}) {
	defer close(eventDoneCh)
	for call := range eventCallCh {
		if err := call.fn(rq, call.msg.Elem, call.msg.What, call.msg.Data); err != nil {
			select {
			case outboundMsgCh <- makeAlertDangerMessage(err):
			default:
				rq.Jaws.Log(fmt.Errorf("jaws: outboundMsgCh full sending event error '%s'", err.Error()))
			}
		}
	}
}

// onConnect calls the Request's ConnectFn if it's not nil, and returns the error from it.
// Returns nil if ConnectFn is nil.
func (rq *Request) onConnect() (err error) {
	rq.mu.RLock()
	connectFn := rq.ConnectFn
	rq.mu.RUnlock()
	if connectFn != nil {
		err = connectFn(rq)
	}
	return
}

func makeAlertDangerMessage(err error) (msg *Message) {
	if err != nil {
		msg = &Message{
			Elem: " alert",
			What: "danger",
			Data: html.EscapeString(err.Error()),
		}
	}
	return
}

// defaultChSize returns a reasonable buffer size for our data channels
func (rq *Request) defaultChSize() (n int) {
	rq.mu.RLock()
	n = 8 + len(rq.elems)*2
	rq.mu.RUnlock()
	return
}
