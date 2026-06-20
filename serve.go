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

// ServeWithTimeout begins processing requests with the given timeout.
// It is intended to run on its own goroutine.
// It returns when [Jaws.Close] is called.
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

	defer func() {
		t.Stop()
		for ch, rq := range subs {
			rq.cancel(nil)
			close(ch)
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
					// the exception is Update messages, more will follow eventually
					if msg.What != what.Update {
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

// Serve calls ServeWithTimeout(DefaultWebSocketTimeout).
// It is intended to run on its own goroutine.
// It returns when [Jaws.Close] is called.
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
	var toLog []error
	jw.mu.Lock()
	nowSeconds := jw.runtimeSeconds.Load()
	for _, rq := range jw.requests {
		if expired, cause := rq.maintenance(nowSeconds, requestTimeout); expired {
			if cause != nil {
				toLog = append(toLog, cause)
			}
			jw.recycleLocked(rq)
		}
	}
	for k, sess := range jw.sessions {
		if sess.isDead() {
			delete(jw.sessions, k)
		}
	}
	jw.mu.Unlock()
	// Log cancellation causes after releasing jw.mu: Jaws.Log calls the
	// user-supplied Logger, which must never run under a core lock.
	for _, cause := range toLog {
		_ = jw.Log(cause)
	}
}
