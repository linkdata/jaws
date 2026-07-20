package jaws

// This file manages the server-side Request pool: NewRequest creates a pending
// Request, UseRequest claims it when the WebSocket connects, the per-IP pending
// limit retires the oldest unclaimed Request, the random helpers mint identity
// keys, and recycle/cancelIfCurrent tear completed Requests down.

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/netip"
	"runtime"
	"slices"
	"time"
	"weak"

	"github.com/linkdata/jaws/lib/key"
)

// NewRequest returns a new JaWS Request.
//
// While the [Jaws] instance is open, the returned Request is pending until it is
// claimed or retired.
//
// Call this as soon as you start processing an HTML request, and store the
// returned [Request] pointer so it can be used while constructing the HTML
// response in order to register the JaWS IDs you use in the response, and
// use its [Request.JawsKey] when sending the JavaScript portion of the reply.
// Do not retain the pointer beyond the initial HTTP handling and rendering; see
// [Request].
//
// Automatic timeout handling is performed by [Jaws.ServeWithTimeout]. The default
// [Jaws.Serve] helper uses a 10-second timeout.
//
// When [Jaws.MaxPendingRequestsPerIP] is positive and already reached,
// NewRequest retires the oldest idle pending Request from the same IP. If every
// pending Request was created or written recently, it retires the oldest one so
// the configured maximum is never exceeded.
//
// A Request created after [Jaws.Close] has an already-canceled context and cannot
// be claimed by [Jaws.UseRequest].
//
// When timeout maintenance or the per-IP pending limit retires an unclaimed
// Request, its key becomes unclaimable. The key also remains unavailable for
// assignment to another Request while the retired Request is reachable; no
// deadline is guaranteed for later reuse.
//
// NewRequest panics if the system CSPRNG ([crypto/rand]) fails while generating
// the request key, which does not happen on supported platforms.
func (jw *Jaws) NewRequest(r *http.Request) (rq *Request) {
	remoteIP := jw.clientIP(r)

	var toLog []error
	func() {
		jw.mu.Lock()
		defer jw.mu.Unlock()
		// Refresh before selecting a pending eviction victim as well as before
		// seeding the new Request. Before Serve starts there is no maintenance loop
		// to advance the counter, so a stale value could otherwise make an idle
		// pending Request look freshly written and lose eviction preference.
		jw.refreshRuntimeSeconds()
		closed := false
		select {
		case <-jw.closeCh:
			closed = true
		default:
			toLog = jw.limitPendingRequestsLocked(remoteIP)
		}
		for rq == nil {
			jawsKey := jw.nonZeroRandomLocked()
			if _, ok := jw.requests[jawsKey]; !ok {
				rq = jw.getRequestLocked(jawsKey, r, remoteIP, !closed)
				if closed {
					rq.cancelFn(nil)
				} else {
					jw.requests[jawsKey] = rq
					jw.requestCount++
					jw.pending[rq.remoteIP] = append(jw.pending[rq.remoteIP], rq)
				}
			}
		}
	}()
	// Log eviction causes after releasing jw.mu: Jaws.Log calls the user-supplied
	// Logger, which must never run under a core lock.
	for _, cause := range toLog {
		_ = jw.Log(cause)
	}
	return
}

// refreshRuntimeSeconds updates runtimeSeconds to the whole seconds elapsed since
// the [Jaws] was created.
//
// Request allocation calls it before pending-request eviction and timestamp
// seeding. The Serve loop also calls it once at start and on every maintenance
// tick, so per-write [Request.MarkWritten] only does an atomic load rather than
// reading the clock.
func (jw *Jaws) refreshRuntimeSeconds() {
	// time.Since on a monotonic base is never negative and keeps the counter immune to
	// wall-clock and NTP adjustments. The int32 conversion is intentionally
	// modulo-style: recency checks compare nearby samples, and their windows are far
	// smaller than 2^31 seconds.
	jw.runtimeSeconds.Store(int32(time.Since(jw.created) / time.Second)) // #nosec G115 -- intentional relative-time counter
}

// limitPendingRequestsLocked evicts pending Requests for remoteIP until the cap is
// satisfied, returning the eviction causes for the caller to log after releasing
// jw.mu (see the package locking contract). Caller must hold jw.mu.
func (jw *Jaws) limitPendingRequestsLocked(remoteIP netip.Addr) (toLog []error) {
	limit := jw.MaxPendingRequestsPerIP
	if limit > 0 {
		nowSeconds := jw.runtimeSeconds.Load()
		for len(jw.pending[remoteIP]) >= limit {
			victim := jw.oldestIdlePendingLocked(remoteIP, nowSeconds)
			if victim == nil {
				// Every pending Request for this IP was written recently. Retire the
				// oldest one so the configured maximum remains a hard cap.
				victim = jw.pending[remoteIP][0]
			}
			if cause := jw.retireNonRunningRequestWithCauseLocked(victim, newErrTooManyPendingRequests(remoteIP, limit)); cause != nil {
				toLog = append(toLog, cause)
			}
		}
	}
	return
}

// oldestIdlePendingLocked returns the oldest pending [Request] for remoteIP that
// was not written recently, or nil if every pending Request was written recently.
// nowSeconds is the reference instant ([Jaws.runtimeSeconds]), passed in so all
// candidates are judged against the same instant. Caller must hold jw.mu.
func (jw *Jaws) oldestIdlePendingLocked(remoteIP netip.Addr, nowSeconds int32) *Request {
	// A recently written Request is skipped while an idle eviction victim exists.
	// RequestWriter.Write records the current second on every write via
	// Request.MarkWritten, so a Request is treated as possibly rendering while its
	// last write is within 2*maintenanceInterval (rounded to whole seconds, with a
	// one-second floor). The recorded second advances only while the Request keeps
	// writing, so an actively writing render stays fresh while one idle for the
	// window is preferred. If all pending Requests are fresh, the caller retires
	// the oldest one to enforce the configured maximum.
	//
	// maintenanceInterval is zero until ServeWithTimeout starts; fall back to
	// DefaultUpdateInterval so an in-flight render is still protected before the
	// maintenance pass begins running. The exact fallback value need not match the
	// steady-state maintenanceInterval: the one-second floor below dominates for any
	// sub-second interval, and a NewRequest before Serve is in any case unusual.
	interval := jw.maintenanceInterval
	if interval <= 0 {
		interval = DefaultUpdateInterval
	}
	spareWindow := 2 * interval
	if spareWindow < time.Second {
		spareWindow = time.Second // floor: the seconds counter advances at most once per second
	}
	for _, rq := range jw.pending[remoteIP] {
		// Compare as durations (elapsed whole seconds vs the window) to avoid a
		// lossy time.Duration conversion. A write timestamp newer than this scan's
		// nowSeconds is fresh; that can happen when a render records a write while
		// the Serve loop's runtimeSeconds snapshot is briefly stale.
		elapsedSeconds := nowSeconds - rq.lastWriteSeconds.Load()
		if elapsedSeconds <= 0 || time.Duration(elapsedSeconds)*time.Second <= spareWindow {
			continue
		}
		return rq
	}
	return nil
}

func (jw *Jaws) removePendingRequestLocked(rq *Request) {
	pending := jw.pending[rq.remoteIP]
	if i := slices.Index(pending, rq); i >= 0 {
		pending = slices.Delete(pending, i, i+1)
		if len(pending) == 0 {
			delete(jw.pending, rq.remoteIP)
		} else {
			jw.pending[rq.remoteIP] = pending
		}
	}
}

func (jw *Jaws) nonZeroRandomUint64Locked() (value uint64) {
	random := make([]byte, 8)
	for value == 0 {
		if _, err := io.ReadFull(jw.kg, random); err != nil {
			panic(err)
		}
		value = binary.LittleEndian.Uint64(random)
	}
	return
}

func (jw *Jaws) nonZeroRandomLocked() key.Key {
	return key.Key(jw.nonZeroRandomUint64Locked())
}

// UseRequest extracts the JaWS [Request] with the given key from the request
// map if it exists and the HTTP request remote IP matches.
//
// Call it when receiving the WebSocket connection on "/jaws/:key" to get the
// associated [Request], and then call its [Request.ServeHTTP] method to process the
// WebSocket messages.
//
// Returns nil if the key was not found, the request was already claimed by an
// earlier WebSocket callback, or the IP doesn't match, in which case you
// should return an HTTP "404 Not Found" status.
//
// The returned pointer is borrowed for WebSocket handling. Do not retain it
// after [Request.ServeHTTP] returns; see [Request].
func (jw *Jaws) UseRequest(jawsKey key.Key, r *http.Request) (rq *Request) {
	if jawsKey != 0 {
		var err error
		jw.mu.Lock()
		if waitingRq, ok := jw.requests[jawsKey]; ok && waitingRq != nil {
			if err = waitingRq.claim(r); err == nil {
				rq = waitingRq
				jw.removePendingRequestLocked(rq)
			}
		}
		jw.mu.Unlock()
		_ = jw.Log(err)
	}
	return
}

// getRequestLocked allocates a Request from the pool for jawsKey. remoteIP is the
// already-resolved client IP for r (see NewRequest, the sole caller), passed in to
// avoid recomputing jw.clientIP(r). attachSession is false after Jaws.Close, when
// the canceled Request is returned without registration. Caller must hold jw.mu.
func (jw *Jaws) getRequestLocked(jawsKey key.Key, r *http.Request, remoteIP netip.Addr, attachSession bool) (rq *Request) {
	rq = jw.reqPool.Get().(*Request)
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.JawsKey = jawsKey
	rq.lastWriteSeconds.Store(jw.runtimeSeconds.Load())
	rq.initial = r
	rq.remoteIP = remoteIP
	rq.ctx, rq.cancelFn = context.WithCancelCause(jw.BaseContext)
	if attachSession && r != nil {
		if sess := jw.getSessionLocked(getCookieSessionsIDs(r.Header, jw.CookieName), rq.remoteIP); sess != nil {
			sess.addRequest(rq)
			rq.session = sess
		}
	}
	return rq
}

type retiredRequestKey struct {
	jw      weak.Pointer[Jaws]
	jawsKey key.Key
}

func releaseRetiredRequestKey(retired retiredRequestKey) {
	if jw := retired.jw.Value(); jw != nil {
		jw.mu.Lock()
		if rq, ok := jw.requests[retired.jawsKey]; ok && rq == nil {
			delete(jw.requests, retired.jawsKey)
		}
		jw.mu.Unlock()
		runtime.KeepAlive(jw)
	}
}

// retireNonRunningRequestLocked cancels and unregisters rq without an error
// cause, clearing, or pooling it. A nil entry keeps its key reserved until a
// runtime cleanup runs after the Request becomes unreachable. Caller must hold
// jw.mu, and rq must not be running.
func (jw *Jaws) retireNonRunningRequestLocked(rq *Request) {
	jw.retireNonRunningRequestCoreLocked(rq, nil, nil)
}

// retireNonRunningRequestWithCauseLocked cancels and unregisters rq with err
// without clearing or pooling it. Caller must hold jw.mu, rq must not be running,
// and err must be non-nil.
func (jw *Jaws) retireNonRunningRequestWithCauseLocked(rq *Request, err error) (cause error) {
	jw.retireNonRunningRequestCoreLocked(rq, err, &cause)
	return
}

// retireNonRunningRequestCoreLocked implements normal and cause-bearing
// retirement. A nil causeOut selects normal cancellation; otherwise err must be
// non-nil and the resulting cancellation cause is stored in causeOut. Caller
// must hold jw.mu, and rq must not be running.
func (jw *Jaws) retireNonRunningRequestCoreLocked(rq *Request, err error, causeOut *error) {
	rq.mu.Lock()
	if rq.JawsKey != 0 && jw.requests[rq.JawsKey] == rq && !rq.running.Load() {
		jawsKey := rq.JawsKey
		if causeOut != nil {
			*causeOut = rq.cancelLocked(err)
		} else if rq.ctx.Err() == nil {
			rq.cancelFn(nil)
		}
		jw.removePendingRequestLocked(rq)
		jw.requests[jawsKey] = nil
		jw.requestCount--
		// Preserve the claimed state until session removal observes it. A claimed
		// WebSocket that never reached ServeHTTP still earns the session grace
		// period granted by Session.delRequest.
		rq.killSessionLocked()
		rq.claimed.Store(false)
		runtime.AddCleanup(rq, releaseRetiredRequestKey, retiredRequestKey{jw: weak.Make(jw), jawsKey: jawsKey})
	}
	rq.mu.Unlock()
	runtime.KeepAlive(rq)
}

// recycleLockedWithCause cancels and recycles rq.
//
// It uses err as the cancellation cause when non-nil.
// It returns the cancellation cause (or nil) instead of logging it, so the caller
// can log it after releasing jw.mu (see the package locking contract). Caller must
// hold jw.mu.
func (jw *Jaws) recycleLockedWithCause(rq *Request, err error) (cause error) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if rq.JawsKey != 0 && jw.requests[rq.JawsKey] == rq {
		cause = rq.cancelLocked(err)
		jw.removePendingRequestLocked(rq)
		delete(jw.requests, rq.JawsKey)
		jw.requestCount--
		rq.clearLocked()
		jw.reqPool.Put(rq)
	}
	return
}

func (jw *Jaws) recycleLocked(rq *Request) {
	_ = jw.recycleLockedWithCause(rq, nil) // nil err yields a nil cause; nothing to log
}

func (jw *Jaws) recycle(rq *Request) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	jw.recycleLocked(rq)
}

// cancelIfCurrent cancels rq only if it is still the [Request] registered for
// jawsKey. A caller that looks up a Request and later cancels it without holding
// jw.mu in between (the /jaws/.tail write-error path in [Jaws.ServeHTTP]) holds
// a pointer that may have been recycled and reused for a different connection,
// and cancelling such a stale pointer would kill the unrelated new request.
// Holding jw.mu across the cancel keeps the identity check valid, since
// recycling requires the jw.mu write lock.
func (jw *Jaws) cancelIfCurrent(jawsKey key.Key, rq *Request, err error) {
	var cause error
	jw.mu.RLock()
	if jw.requests[jawsKey] == rq {
		rq.mu.Lock()
		cause = rq.cancelLocked(err)
		rq.mu.Unlock()
	}
	jw.mu.RUnlock()
	// Log after releasing both locks: Jaws.Log calls the user-supplied Logger,
	// which must never run under a core lock (this path holds jw.mu read).
	_ = jw.Log(cause)
}
