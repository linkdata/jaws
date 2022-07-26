// Package jaws provides a mechanism to create dynamic
// webpages using Javascript and WebSockets.
//
// It integrates well with Go's html/template package,
// but can be used without it. It can be used with any
// router that supports the standard ServeHTTP interface.
//
// It comes with a small package 'jawsecho' that
// integrates with Echo and also doubles as an example
// for integration with other routers.
package jaws

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Jaws struct {
	Logger  *log.Logger // If not nil, send debug info and errors here
	doneCh  <-chan struct{}
	bcastCh chan *Message
	subCh   chan chan *Message
	unsubCh chan chan *Message
	kg      *Keygen
	nextId  uint64
	mu      sync.Mutex // protects following
	closeCh chan struct{}
	reqs    map[int64]*Request
}

// NewWithDone returns a new JaWS object using the given completion channel.
// This is expected to be created once per HTTP server and handles
// publishing HTML changes across all connections.
func NewWithDone(doneCh <-chan struct{}) *Jaws {
	return &Jaws{
		doneCh:  doneCh,
		bcastCh: make(chan *Message, 1),
		subCh:   make(chan chan *Message, 1),
		unsubCh: make(chan chan *Message, 1),
		kg:      NewKeygen(),
		reqs:    make(map[int64]*Request),
	}
}

// New returns a new JaWS object that must be closed using Close().
// This is expected to be created once per HTTP server and handles
// publishing HTML changes across all connections.
func New() (jw *Jaws) {
	closeCh := make(chan struct{})
	jw = NewWithDone(closeCh)
	jw.closeCh = closeCh
	return
}

// Close frees resources associated with the JaWS object, and
// closes the completion channel if the JaWS was created with New().
// Once the completion channel is closed, broadcasts and sends are discarded.
// Subsequent calls to Close() have no effect.
func (jw *Jaws) Close() {
	jw.mu.Lock()
	if jw.closeCh != nil {
		close(jw.closeCh)
		jw.closeCh = nil
	}
	jw.mu.Unlock()
}

// Done returns the completion channel.
func (jw *Jaws) Done() <-chan struct{} {
	return jw.doneCh
}

// Log sends an error to the Logger set in the Jaws.
// Has no effect if the err is nil or the Logger is nil.
// Returns err.
func (jw *Jaws) Log(err error) error {
	if err != nil && jw != nil && jw.Logger != nil {
		jw.Logger.Println(err.Error())
	}
	return err
}

// MustLog sends an error to the Logger set in the Jaws or
// panics with the given error if no Logger is set.
// Has no effect if the err is nil.
func (jw *Jaws) MustLog(err error) {
	if err != nil {
		if jw != nil && jw.Logger != nil {
			jw.Logger.Println(err.Error())
		} else {
			panic(err)
		}
	}
}

// MakeID returns a string in the form 'jaws.X' where X is a string unique within the Jaws lifetime.
func (jw *Jaws) MakeID() string {
	return "jaws." + strconv.FormatUint(atomic.AddUint64(&jw.nextId, 1), 32)
}

// NewRequest returns a new JaWS request.
// Call this as soon as you start processing a HTML request, and store the returned Request pointer so it can be used
// while constructing the HTML response in order to register the HTML entity id's you use in the response, and use
// it's Key attribute when sending the Javascript portion of the reply with GetBodyFooter.
//
// Don't use the http.Request's Context, as that will expire before the WebSocket call comes in.
func (jw *Jaws) NewRequest(ctx context.Context, remoteAddr string) (rq *Request) {
	for rq == nil {
		jawsKey := jw.kg.Int63()
		jw.mu.Lock()
		if _, ok := jw.reqs[jawsKey]; !ok {
			rq = newRequest(ctx, jw, jawsKey, remoteAddr)
			jw.reqs[jawsKey] = rq
		}
		jw.mu.Unlock()
	}
	return
}

// UseRequest removes the JaWS request with the given key from the request
// map if it exists and the remoteAddr matches, and if so returns the Request.
//
// Call it when receiving the WebSocket connection on '/jaws/:key' to get the
// associated Request, and then call it's ServeHTTP method to process the
// WebSocket messages.
//
// Returns nil if the key was not found, in which case you should return a HTTP 404 Not Found code.
func (jw *Jaws) UseRequest(jawsKey int64, remoteAddr string) (rq *Request) {
	var ok bool
	jw.mu.Lock()
	if rq, ok = jw.reqs[jawsKey]; ok {
		remoteIP := parseIP(remoteAddr)
		if (remoteIP == nil && rq.remoteIP == nil) || remoteIP.Equal(rq.remoteIP) {
			delete(jw.reqs, jawsKey)
		} else {
			jw.Log(fmt.Errorf("%v: expected IP %v, got %v", rq, rq.remoteIP, remoteIP))
			rq = nil
		}
	}
	jw.mu.Unlock()
	return
}

// Broadcast sends a message to all Requests.
func (jw *Jaws) Broadcast(msg *Message) {
	select {
	case <-jw.Done():
	case jw.bcastCh <- msg:
	}
}

// SetInner sends an HTML id and new inner HTML to all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) SetInner(id string, innerHtml string) {
	jw.Broadcast(&Message{
		Elem: id,
		What: "inner",
		Data: innerHtml,
	})
}

// Remove removes the HTML element with the given ID on all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Remove(id string) {
	jw.Broadcast(&Message{
		Elem: id,
		What: "remove",
	})
}

// Insert calls the Javascript 'insertBefore()' method on the given element on all Requests.
// The position parameter 'where' may be either a HTML ID, an child index or the text 'null'.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Insert(parentId, where, html string) {
	jw.Broadcast(&Message{
		Elem: parentId,
		What: "insert",
		Data: where + "\n" + html,
	})
}

// Append calls the Javascript 'appendChild()' method on the given element on all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Append(parentId, html string) {
	jw.Broadcast(&Message{
		Elem: parentId,
		What: "append",
		Data: html,
	})
}

// Replace calls the Javascript 'replaceChild()' method on the given element on all Requests.
// The position parameter 'where' may be either a HTML ID or an index.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Replace(id, where, html string) {
	jw.Broadcast(&Message{
		Elem: id,
		What: "replace",
		Data: where + "\n" + html,
	})
}

// SetAttr sends an HTML id and new attribute value to all Requests.
// If the value is an empty string, a value-less attribute will be added (such as "disabled")
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) SetAttr(id, attr, val string) {
	jw.Broadcast(&Message{
		Elem: id,
		What: "sattr",
		Data: attr + "\n" + val,
	})
}

// RemoveAttr removes a given attribute from the HTML id for all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) RemoveAttr(id, attr string) {
	jw.Broadcast(&Message{
		Elem: id,
		What: "rattr",
		Data: attr,
	})
}

// SetValue sends an HTML id and new input value to all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) SetValue(id, val string) {
	jw.Broadcast(&Message{
		Elem: id,
		What: "value",
		Data: val,
	})
}

// Reload requests all Requests to reload their current page.
func (jw *Jaws) Reload() {
	jw.Broadcast(&Message{
		Elem: " reload",
	})
}

// Redirect requests all Requests to navigate to the given URL.
func (jw *Jaws) Redirect(url string) {
	jw.Broadcast(&Message{
		Elem: " redirect",
		What: url,
	})
}

// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests.
func (jw *Jaws) Trigger(id, val string) {
	jw.Broadcast(&Message{
		Elem: id,
		What: "trigger",
		Data: val,
	})
}

// Alert sends an alert to all Requests. The lvl argument should be one of Bootstraps alert levels:
// primary, secondary, success, danger, warning, info, light or dark.
func (jw *Jaws) Alert(lvl, msg string) {
	jw.Broadcast(&Message{
		Elem: " alert",
		What: lvl,
		Data: msg,
	})
}

// Count returns the number of requests waiting for their WebSocket callbacks.
func (jw *Jaws) Pending() (n int) {
	jw.mu.Lock()
	n = len(jw.reqs)
	jw.mu.Unlock()
	return
}

// ServeWithTimeout begins processing requests with the given timeout.
// It is intended to run on it's own goroutine.
// It returns when the completion channel is closed.
func (jw *Jaws) ServeWithTimeout(requestTimeout time.Duration) {
	const minInterval = time.Millisecond * 10
	const maxInterval = time.Second
	maintenanceInterval := requestTimeout / 2
	if maintenanceInterval > maxInterval {
		maintenanceInterval = maxInterval
	}
	if maintenanceInterval < minInterval {
		maintenanceInterval = minInterval
	}
	t := time.NewTicker(maintenanceInterval)
	defer t.Stop()
	subs := map[chan *Message]struct{}{}
	for {
		select {
		case <-jw.Done():
			return
		case <-t.C:
			if jw.kg.IsUsed() {
				jw.kg.Reseed()
			}
			jw.maintenance(requestTimeout)
		case msgCh := <-jw.subCh:
			if msgCh != nil {
				subs[msgCh] = struct{}{}
			}
		case msgCh := <-jw.unsubCh:
			if _, ok := subs[msgCh]; ok {
				delete(subs, msgCh)
				close(msgCh)
			}
		case msg := <-jw.bcastCh:
			if msg != nil {
				for msgCh := range subs {
					select {
					case msgCh <- msg:
					default:
						// it's critical that we keep the broadcast
						// distribution loop running, so any Request
						// that fails to process it's messages quickly
						// enough must be terminated. the alternative
						// would be to drop some messages, but that
						// could mean nonreproducible and seemingly
						// random failures in processing logic.
						close(msgCh)
						delete(subs, msgCh)
						jw.Log(fmt.Errorf("jaws: broadcast channel full sending %v", msg))
					}
				}
			}
		}
	}
}

// Serve calls ServeWithTimeout(time.Second * 10).
func (jw *Jaws) Serve() {
	jw.ServeWithTimeout(time.Second * 10)
}

func (jw *Jaws) subscribe(size int) chan *Message {
	msgCh := make(chan *Message, size)
	select {
	case <-jw.Done():
		close(msgCh)
		return nil
	case jw.subCh <- msgCh:
	}
	return msgCh
}

func (jw *Jaws) unsubscribe(msgCh chan *Message) {
	select {
	case <-jw.Done():
	case jw.unsubCh <- msgCh:
	}
}

func (jw *Jaws) maintenance(requestTimeout time.Duration) {
	var toCycle []*Request
	deadline := time.Now().Add(-requestTimeout)
	jw.mu.Lock()
	for k, rq := range jw.reqs {
		if rq.Started.Before(deadline) {
			delete(jw.reqs, k)
			toCycle = append(toCycle, rq)
		}
	}
	jw.mu.Unlock()
	for _, rq := range toCycle {
		rq.recycle()
	}
}

func parseIP(remoteAddr string) (ip net.IP) {
	if remoteAddr != "" {
		if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
			ip = net.ParseIP(host)
		} else {
			ip = net.ParseIP(remoteAddr)
		}
	}
	return
}
