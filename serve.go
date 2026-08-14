package jaws

// This file implements the JaWS processing loop. ServeWithTimeout runs the
// select loop that distributes broadcasts to subscribed Requests and drives
// periodic maintenance; Serve, subscribe, unsubscribe and maintenance support it.

import (
	"fmt"
	"time"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// Pending returns the number of requests waiting for their WebSocket callbacks.
func (jw *Jaws) Pending() (n int) {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	for _, pending := range jw.pending {
		n += len(pending)
	}
	return
}

func (jw *Jaws) getWebSocketTimeout() (t time.Duration) {
	jw.mu.RLock()
	t = jw.webSocketTimeout
	jw.mu.RUnlock()
	return
}

// ServeWithTimeout begins processing requests.
//
// requestTimeout must be an exact multiple of [time.Second] from [time.Second]
// through 2,147,483,646 seconds. Other values have unspecified behavior.
//
// An overlapping [Jaws.Serve] or [Jaws.ServeWithTimeout] call reports
// [ErrServeAlreadyRunning] through [Jaws.MustLog]. It panics without a Logger
// and in debug or race builds; otherwise it returns without starting another
// processing loop.
//
// Before [Request.ServeHTTP] begins WebSocket processing, timeout-based Request
// retirement is periodic and approximate, not a hard deadline. [Jaws.NewRequest],
// a successful [Jaws.UseRequest], and [Request.MarkWritten] mark activity using
// whole-second samples from the epoch established by [New]. Retirement is checked
// only during maintenance passes, so it is not timed precisely from those events.
//
// A WebSocket read that remains pending for [Jaws.WebSocketPingInterval] triggers
// a keepalive ping. requestTimeout bounds each ping. Data or a successful ping
// starts a new interval; time spent delivering an already-read message for
// processing does not count as read-idle time. This timing does not use the
// initial-render activity samples or maintenance schedule.
//
// It is intended to run on its own goroutine and returns when [Jaws.Close] is
// called. Errors reported through [Jaws.Log] are queued without waiting for
// Logger.Error. On a normal return after shutdown, ServeWithTimeout waits for
// every log entry accepted before [Jaws.Close] to finish. A blocked Logger.Error
// callback therefore delays that return.
func (jw *Jaws) ServeWithTimeout(requestTimeout time.Duration) {
	if !jw.serving.CompareAndSwap(false, true) {
		jw.reportMisuse(ErrServeAlreadyRunning)
		return
	}
	defer jw.serving.Store(false)

	const minInterval = time.Millisecond * 10
	const maxInterval = time.Second
	maintenanceInterval := min(requestTimeout/2, maxInterval)
	maintenanceInterval = max(maintenanceInterval, minInterval)

	subs := map[chan wire.Message]*Request{}
	t := time.NewTicker(maintenanceInterval)
	jw.mu.Lock()
	jw.webSocketTimeout = requestTimeout
	jw.maintenanceInterval = maintenanceInterval
	jw.mu.Unlock()
	// Seed the seconds counter so it is accurate from the first request, then keep
	// it fresh on every maintenance tick (see the case below).
	jw.refreshRuntimeSeconds()

	normalShutdown := false
	defer func() {
		t.Stop()
		for ch, rq := range subs {
			rq.cancel(nil)
			close(ch)
		}
		// Only the Done case below is a normal shutdown. A panic can race Close;
		// waiting here while it unwinds could hide that panic behind a blocked
		// Logger.Error callback. The flag preserves panic and Goexit semantics
		// without recover and re-panic.
		if normalShutdown && jw.loggerQueue != nil {
			<-jw.loggerQueue.doneCh
		}
	}()

	killSub := func(msgCh chan wire.Message) {
		if _, ok := subs[msgCh]; ok {
			delete(subs, msgCh)
			close(msgCh)
		}
	}

	// it is critical that we keep the broadcast
	// distribution loop running, so any Request
	// that fails to process its messages quickly
	// enough must be terminated. the alternative
	// would be to drop some messages, but that
	// could mean nonreproducible and seemingly
	// random failures in processing logic.
	mustBroadcast := func(msg wire.Message) {
		for msgCh, rq := range subs {
			if msg.Dest == nil || rq.wantMessage(&msg) {
				select {
				case msgCh <- msg:
				default:
					// Only the internal periodic dirty-render tick, a nil-destination
					// Update (see the updateTicker case below), is safe to drop.
					// distributeDirt has already moved the dirty selectors into Requests'
					// pending-dirt lists and cleared the global set, so the
					// tick carries no payload;
					// it only nudges the Request. The pending dirt is still rendered
					// without it: a Request already in its process loop is woken by the
					// message that filled the channel and drains todoDirt on the next pass,
					// and one still starting up (subscribed before onConnect) drains
					// todoDirt on its first pass without needing a wake. Every addressed
					// message is one-shot and must not be silently dropped — including a
					// tag-targeted Update and the key-targeted Update wake-up from
					// Session.Close — so an overloaded Request is failed-fast instead.
					if msg.What != what.Update || msg.Dest != nil {
						killSub(msgCh)
						rq.cancel(fmt.Errorf("%w: %v: broadcast channel full sending %s", ErrRequestOverloaded, rq, msg.String()))
					}
				}
			}
		}
	}

	for {
		select {
		case <-jw.Done():
			normalShutdown = true
			return
		case <-jw.updateTicker.C:
			if jw.distributeDirt() > 0 {
				mustBroadcast(wire.Message{What: what.Update})
			}
		case <-t.C:
			jw.refreshRuntimeSeconds()
			jw.maintenance(requestTimeout)
		case sub := <-jw.subCh:
			if sub.msgCh != nil {
				subs[sub.msgCh] = sub.rq
			}
		case msgCh := <-jw.unsubCh:
			killSub(msgCh)
		case msg, ok := <-jw.bcastCh:
			if ok {
				mustBroadcast(msg)
			}
		}
	}
}

// Serve calls [Jaws.ServeWithTimeout] with [DefaultWebSocketTimeout].
//
// See [Jaws.ServeWithTimeout] for lifecycle and panic behavior.
func (jw *Jaws) Serve() {
	jw.ServeWithTimeout(DefaultWebSocketTimeout)
}

func (jw *Jaws) subscribe(rq *Request, size int) chan wire.Message {
	msgCh := make(chan wire.Message, size)
	select {
	case <-jw.Done():
		close(msgCh)
		return nil
	case jw.subCh <- subscription{msgCh: msgCh, rq: rq}:
	}
	return msgCh
}

func (jw *Jaws) unsubscribe(msgCh chan wire.Message) {
	select {
	case <-jw.Done():
	case jw.unsubCh <- msgCh:
	}
}

func (jw *Jaws) maintenance(requestTimeout time.Duration) {
	jw.mu.Lock()
	nowSeconds := jw.runtimeSeconds.Load()
	for _, rq := range jw.requests {
		if rq == nil {
			continue
		}
		if expired, cause := rq.maintenance(nowSeconds, requestTimeout); expired {
			_ = jw.Log(cause)
			jw.retireNonRunningRequestLocked(rq)
		}
	}
	for k, sess := range jw.sessions {
		if sess.isDead() {
			delete(jw.sessions, k)
		}
	}
	jw.mu.Unlock()
}
