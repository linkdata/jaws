package jaws

import (
	"time"

	"github.com/linkdata/jaws/lib/wire"
)

// TestServe runs rq's WebSocket message-processing loop for test harnesses,
// including the out-of-package harness in github.com/linkdata/jaws/jawstest.
//
// It subscribes rq to broadcasts, waits for the running Serve loop to process
// the subscription, transitions rq to running with the same checked transition
// [Request.ServeHTTP] uses, then runs rq.process in a new goroutine using freshly
// created inbound/outbound channels, recycling rq when the loop stops.
//
// rq must already be claimed via [Jaws.UseRequest]. TestServe panics — like its other
// setup-failure panics — if the Jaws processing loop ([Jaws.Serve] or
// [Jaws.ServeWithTimeout]) is not running, or if rq is not servable (unclaimed,
// already being served, retired, or the instance is closed).
//
// TestServe is exported solely to let test harnesses outside package jaws drive a
// request loop without access to unexported internals. It is not intended for
// production use; it does not import any testing-only packages, so it does not
// pull net/http/httptest into the production build.
//
// onPanic must be non-nil; it is called with the recovered value (nil if the loop
// exited normally) when the loop goroutine stops, before doneCh is closed, so a
// harness can publish captured panic state before any <-doneCh waiter observes
// it. A harness that does not expect panics should re-panic when the value is
// non-nil so unexpected loop panics still surface.
func (jw *Jaws) TestServe(rq *Request, onPanic func(recovered any)) (inCh chan wire.WsMsg, outCh chan wire.WsMsg, bcastCh chan wire.Message, readyCh, doneCh chan struct{}) {
	bcastCh = make(chan wire.Message, 64)
	// Subscribe and then rendezvous with the Serve loop so the subscription is
	// installed before the test request starts processing. This requires
	// Serve/ServeWithTimeout to be running; if it is not, fail loudly with a clear
	// message instead.
	select {
	case jw.subCh <- subscription{msgCh: bcastCh, rq: rq}:
	case <-jw.Done():
		close(bcastCh)
		panic("jaws: TestServe: the Jaws instance is closed")
	case <-time.After(5 * time.Second):
		close(bcastCh)
		panic("jaws: TestServe timed out subscribing; the Jaws processing loop (Serve or ServeWithTimeout) must be running")
	}
	select {
	case jw.subCh <- subscription{}:
	case <-jw.Done():
		panic("jaws: TestServe: the Jaws instance is closed")
	case <-time.After(5 * time.Second):
		panic("jaws: TestServe timed out subscribing; the Jaws processing loop (Serve or ServeWithTimeout) must be running")
	}

	// Transition to running with the same checked transition Request.ServeHTTP uses,
	// synchronously and before creating the per-run channels. casState(reqClaimed,
	// reqRunning) requires the Request to be claimed (via UseRequest) and not already
	// running, retired, or closed, so a concurrent Jaws.Close/retirement or a second
	// TestServe/ServeHTTP cannot resurrect it into reqRunning. On failure, release the
	// subscription and panic (like TestServe's other setup-failure panics) rather than
	// return a half-started harness or recycle a Request another owner is serving.
	if !rq.startServe() {
		jw.unsubscribe(bcastCh)
		panic("jaws: TestServe: request is not servable; claim it via UseRequest first, and serve it only once and before Close")
	}

	inCh = make(chan wire.WsMsg)
	outCh = make(chan wire.WsMsg, cap(bcastCh))
	readyCh = make(chan struct{})
	doneCh = make(chan struct{})

	go func() {
		// onPanic runs before doneCh closes so a harness can publish its captured
		// panic state before any <-doneCh waiter observes it. onPanic may re-panic
		// to propagate an unexpected panic; that skips the close, which is moot
		// since the goroutine is then crashing the test anyway.
		defer func() {
			onPanic(recover())
			close(doneCh)
		}()
		close(readyCh)
		rq.process(bcastCh, inCh, outCh)
		jw.recycle(rq)
	}()
	return
}
