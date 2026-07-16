package jaws

// This file implements the per-request runtime: the message-processing loop that
// runs on a Request's own goroutines while its WebSocket is connected.
//
// [Request.process] is the select loop. Inbound client messages are dispatched by
// callAllEventHandlers and executed on the event goroutine via eventCaller;
// broadcasts and tag messages are applied by handleBroadcast and handleRemove;
// dirty elements are rendered into the outbound queue by getSendMsgs, sendQueue
// and makeUpdateList. onConnect runs the user ConnectFn once the socket is up.

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

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
	case key.Key:
		// request-targeted; page-global What values (Reload/Redirect/Order/Alert)
		// already returned above, so there are no elements to resolve here
	case string:
		// target is a regular HTML ID. With Jid < 0, wire.WsMsg.Append writes Data
		// verbatim and the browser splits the frame on '\t' (fields) and '\n' (frames),
		// so an id carrying either byte corrupts the frame (a '\n' splits it in two).
		// The data half below is JSON-quoted, but the id is concatenated raw, so guard
		// it: a valid HTML id never contains ASCII whitespace, so reject rather than
		// escape, mirroring how Element.Replace rejects an id-less payload.
		// Page/HTML-id-targeted commands (including Delete) only emit a DOM frame and
		// never mutate the server-side Element/tag registry, unlike element/tag-targeted
		// Delete which also removes the Element via DeleteElement below.
		if strings.ContainsAny(v, "\t\n") {
			rq.Jaws.reportMisuse(fmt.Errorf("jaws: Broadcast: HTML id %q contains a tab or newline", v))
			return
		}
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
			// Hook messages synchronously invoke the element's event handler; see
			// [what.Hook]. They exist for testing: the JaWS client never sends Hook,
			// so they only arrive here via Broadcast. The handler must not send any
			// messages itself, but may return an error, which is sent back to the
			// client as an alert message.
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
	// Incoming what.Remove messages from jaws.js are cleanup acknowledgements sent
	// while applying server-driven DOM mutation commands (Inner, Replace, Delete or
	// Remove). Data is a tab-separated list of managed descendant IDs that were
	// removed from the DOM. A positive WebSocket Jid identifies a managed
	// parent/container and must not itself be deleted here; Jid zero means the
	// container has an ordinary HTML id and is not registered on the Request.
	//
	// The client is already trusted only within its own request: a malicious client
	// can fully control the DOM and UI it presents to its user. Treating arbitrary
	// child removals as request-local state cleanup is therefore not a server-side
	// privilege boundary; IDs are only looked up in this Request.
	if containerJid.IsValid() {
		rq.mu.Lock()
		defer rq.mu.Unlock()
		// Collect the requested child elements, then delete them in a single pass
		// over rq.elems and rq.tagMap, rather than an O(N) scan plus O(N) compaction
		// per id (the id count is client-controlled, bounded by the read limit).
		var victims map[Jid]struct{}
		for jidstr := range strings.SplitSeq(data, "\t") {
			if id := jid.ParseString(jidstr); id != containerJid {
				if e := rq.getElementByJidLocked(id); e != nil {
					if victims == nil {
						victims = map[Jid]struct{}{}
					}
					e.deleted.Store(true)
					victims[e.Jid()] = struct{}{}
				}
			}
		}
		if len(victims) == 0 {
			return
		}
		rq.removeElementsLocked(func(e *Element) bool { _, ok := victims[e.Jid()]; return ok })
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
// element. Only frozen Elements are selected, so the atomic frozen load publishes
// the completed handler slice before it is read without a lock. ErrEventUnhandled
// is normalized to nil. rq.mu is held only for the element lookups; the handlers
// themselves run unlocked.
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
					if e := rq.getElementByJidLocked(id); e != nil && !e.deleted.Load() && e.frozen.Load() {
						elems = append(elems, e)
					}
				}
			}
		}
	} else {
		if e := rq.getElementByJidLocked(id); e != nil && !e.deleted.Load() && e.frozen.Load() {
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
		rq.cancel(fmt.Errorf("%w: %v: eventCallCh full sending %v", ErrRequestOverloaded, rq, call))
	}
}

// getSendMsgs drains the pending wsQueue, dropping messages addressed to elements
// that are not present (non-element messages and Delete are always kept), and
// returns the survivors sorted by Jid. It takes rq.mu (read) then muQueue, the
// order required by the lock hierarchy documented in jaws.go.
//
// what.Order is page-global (the browser ignores its Jid; see jawsPerform), but
// Element.Order queues it carrying the issuing element's Jid so the stable Jid sort
// places it after that element's own Append frames. It is therefore kept regardless
// of whether the issuer is still present, like the other page-global commands.
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
			ok := rq.wsQueue[i].Jid < 1 || rq.wsQueue[i].What == what.Delete || rq.wsQueue[i].What == what.Order
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

// removeElementsLocked drops every element matching pred from the request's element
// list and from every tag entry, deleting tag entries that become empty.
//
// slices.DeleteFunc zeros the freed tail slots, so the dropped *Element pointers do
// not linger in the backing arrays. Caller must hold rq.mu and is responsible for
// marking the matched elements deleted.
func (rq *Request) removeElementsLocked(pred func(*Element) bool) {
	rq.elems = slices.DeleteFunc(rq.elems, pred)
	for k := range rq.tagMap {
		rq.tagMap[k] = slices.DeleteFunc(rq.tagMap[k], pred)
		if len(rq.tagMap[k]) == 0 {
			delete(rq.tagMap, k)
		}
	}
}

// deleteElementLocked removes elem from the request's element list and from every
// tag entry, marking it deleted; it is a no-op if elem belongs to another request.
// Caller must hold rq.mu.
func (rq *Request) deleteElementLocked(elem *Element) {
	if elem.Request == rq {
		elem.deleted.Store(true)
		rq.removeElementsLocked(func(e *Element) bool { return e == elem })
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

// eventCaller calls event functions.
//
// Once the Request context is cancelled it stops invoking handlers and drains the
// remaining queued calls as no-ops. This cannot strand events on a still-live
// Request: process selects on the same rq.Context().Done() and returns, then closes
// eventCallCh, so the no-op drain is always part of teardown.
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
			// This error alert is best-effort: unlike queueEvent, which cancels the
			// Request with ErrRequestOverloaded when its channel fills (dropping a
			// queued event could desync browser and backend state), a dropped alert
			// loses no state — the underlying error is already logged below — so a
			// full outbound channel here is logged and the alert is discarded rather
			// than tearing down the Request.
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
