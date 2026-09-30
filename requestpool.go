package jaws

// This file manages server-side Request lifecycles: NewRequest creates a pending
// Request with a fresh, never-reused identity and reusable buffers, UseRequest
// claims it when the WebSocket connects, the pending cap retires the
// oldest unclaimed Request, the random helpers mint identity keys, and
// recycle/cancelIfCurrent finish completed Requests, returning only their buffers
// to the pool.

import (
	"context"
	"encoding/binary"
	"errors"
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
// NewRequest replaces w's Cache-Control header with "no-store". Call it with
// the response writer before writing its headers or body. Calling it after the
// response is committed does not change the sent headers.
//
// Use the returned [Request] while rendering the initial response to register
// JaWS IDs and write [Request.HeadHTML]. Do not retain it after initial request
// handling and rendering; see [Request].
//
// If r is nil, the Request has no initial request, client address, or [Session].
//
// [Jaws.ServeWithTimeout] periodically retires idle Requests before WebSocket
// processing starts; [Jaws.Serve] uses [DefaultWebSocketTimeout].
//
// When [Jaws.MaxPendingRequestsPerIP] is positive and a bucket is full,
// NewRequest retires the oldest idle pending Request from that bucket. If every
// pending Request was created or written recently, it retires the least recently
// written one so the maximum is never exceeded.
//
// Clients in the same bucket share the limit and eviction pool, including those
// behind a shared NAT or a proxy without [Jaws.TrustForwardedHeaders].
//
// A Request created after [Jaws.Close] has an already-canceled context and cannot
// be claimed by [Jaws.UseRequest].
//
// Every call returns a distinct Request identity that is never reused for another
// connection. When timeout maintenance or the pending cap retires an
// unclaimed Request, its key remains unavailable for assignment to another Request
// while the retired Request is reachable; no deadline is guaranteed for later key
// reuse.
//
// It panics if the [crypto/rand.Reader] captured by [New] returns an error while
// generating the request key. Go's default reader does not return errors.
func (jw *Jaws) NewRequest(w http.ResponseWriter, r *http.Request) *Request {
	// Page metadata carries a one-use Request key. Replaying it from an HTTP
	// cache reuses the consumed key and prevents another WebSocket connection.
	w.Header().Set("Cache-Control", headerCacheControlNoStore)
	return jw.newRequest(r)
}

var wellKnownNAT64Prefix = netip.MustParsePrefix("64:ff9b::/96")

// clientBucketKey uses the embedded IPv4 address for the well-known NAT64 prefix.
// Other IPv6 addresses share a /64; IPv4 addresses use their full address.
func clientBucketKey(addr netip.Addr) netip.Addr {
	addr = addr.Unmap()
	if wellKnownNAT64Prefix.Contains(addr) {
		a := addr.As16()
		return netip.AddrFrom4([4]byte(a[12:]))
	}
	if addr.Is6() {
		return netip.PrefixFrom(addr, 64).Masked().Addr()
	}
	return addr
}

func (jw *Jaws) newRequest(r *http.Request) (rq *Request) {
	remoteIP := jw.clientIP(r)
	bucketKey := clientBucketKey(remoteIP)

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
			jw.limitPendingRequestsLocked(bucketKey)
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
					jw.pending[bucketKey] = append(jw.pending[bucketKey], rq)
					jw.markStatusDirty(StatusMetricPendingRequests)
				}
			}
		}
	}()
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

// limitPendingRequestsLocked evicts pending Requests from bucketKey until
// the cap is satisfied. Caller must hold jw.mu.
func (jw *Jaws) limitPendingRequestsLocked(bucketKey netip.Addr) {
	// Evicting rather than refusing a newcomer keeps a stalled client from
	// blocking the bucket until timeout. See "Pending-cap availability tradeoff"
	// in AI.md.
	limit := jw.MaxPendingRequestsPerIP
	if limit > 0 {
		nowSeconds := jw.runtimeSeconds.Load()
		for len(jw.pending[bucketKey]) >= limit {
			before := len(jw.pending[bucketKey])
			victim := jw.pendingEvictionVictimLocked(bucketKey, nowSeconds)
			if cause := jw.retireNonRunningRequestLocked(victim, newErrTooManyPendingRequests(bucketKey, limit)); cause != nil {
				_ = jw.Log(cause)
			}
			if len(jw.pending[bucketKey]) >= before {
				// Retirement declines a running Request or one that lost registry
				// identity. Neither can be pending, but if that invariant ever broke
				// the loop would reselect the same victim forever while holding jw.mu,
				// so accept an overshoot instead.
				break
			}
		}
	}
}

// pendingEvictionVictimLocked returns the pending [Request] for bucketKey to
// retire when the pending cap is reached: the oldest one that was not written
// recently, or the least recently written one when every pending Request is
// fresh. nowSeconds is the reference instant ([Jaws.runtimeSeconds]), passed in
// so all candidates are judged against the same instant. Caller must hold jw.mu,
// and jw.pending[bucketKey] must be non-empty.
func (jw *Jaws) pendingEvictionVictimLocked(bucketKey netip.Addr, nowSeconds int32) (victim *Request) {
	// A recently written Request is skipped while an idle eviction victim exists.
	// RequestWriter.Write records the current second on every write via
	// Request.MarkWritten, so a Request is treated as possibly rendering while its
	// last write is within 2*maintenanceInterval (rounded to whole seconds, with a
	// one-second floor). The recorded second advances only while the Request keeps
	// writing, so an actively writing render stays fresh while one idle for the
	// window is preferred. If all pending Requests are fresh, the least recently
	// written one is retired to enforce the configured maximum.
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
	var victimElapsed int32
	for _, rq := range jw.pending[bucketKey] {
		// Compare as durations (elapsed whole seconds vs the window) to avoid a
		// lossy time.Duration conversion. A write timestamp newer than this scan's
		// nowSeconds is fresh; that can happen when a render records a write while
		// the Serve loop's runtimeSeconds snapshot is briefly stale.
		elapsedSeconds := nowSeconds - rq.lastWriteSeconds.Load()
		if elapsedSeconds > 0 && time.Duration(elapsedSeconds)*time.Second > spareWindow {
			// The oldest idle pending Request; pending is in creation order.
			return rq
		}
		if victim == nil || elapsedSeconds > victimElapsed {
			// Least recently written so far; ties keep the oldest-created one.
			victim, victimElapsed = rq, elapsedSeconds
		}
	}
	return
}

func (jw *Jaws) removePendingRequestLocked(rq *Request) {
	bucketKey := clientBucketKey(rq.remoteIP)
	pending := jw.pending[bucketKey]
	if i := slices.Index(pending, rq); i >= 0 {
		pending = slices.Delete(pending, i, i+1)
		if len(pending) == 0 {
			delete(jw.pending, bucketKey)
		} else {
			jw.pending[bucketKey] = pending
		}
		jw.markStatusDirty(StatusMetricPendingRequests)
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
// A successful claim marks activity used by [Jaws.ServeWithTimeout] while the
// Request waits for [Request.ServeHTTP].
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
		func() {
			jw.mu.Lock()
			defer jw.mu.Unlock()
			if waitingRq, ok := jw.requests[jawsKey]; ok && waitingRq != nil {
				if err = waitingRq.claim(r); err == nil {
					rq = waitingRq
					jw.removePendingRequestLocked(rq)
				}
			}
		}()
		// Cancellation was reported when the Request was canceled. Repeated
		// claims must not re-log its initial URI.
		if !errors.Is(err, ErrRequestCancelled) {
			_ = jw.Log(err)
		}
	}
	return
}

// getRequestLocked allocates a fresh Request identity for jawsKey, borrowing
// reusable storage from jw.requestBufferPool. remoteIP is the already-resolved
// client IP for r (see newRequest, the sole caller), passed in to avoid recomputing
// jw.clientIP(r). registered is false after Jaws.Close, when the canceled Request
// is returned without being registered in jw.requests. Caller must hold jw.mu.
func (jw *Jaws) getRequestLocked(jawsKey key.Key, r *http.Request, remoteIP netip.Addr, registered bool) (rq *Request) {
	buffers := jw.requestBufferPool.Get().(*requestBuffers)
	rq = &Request{
		Jaws:     jw,
		buffers:  buffers,
		todoDirt: buffers.todoDirt,
		elems:    buffers.elems,
		tagMap:   buffers.tagMap,
		wsQueue:  buffers.wsQueue,
	}
	// Detach the storage from the pooled holder; releaseBuffersLocked reattaches it
	// on completion. The holder must not alias a live Request's fields.
	buffers.todoDirt = nil
	buffers.elems = nil
	buffers.tagMap = nil
	buffers.wsQueue = nil
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.JawsKey = jawsKey
	if registered {
		rq.storeState(reqPending)
	} else {
		rq.storeState(reqUnclaimable) // created after Jaws.Close: canceled, not claimable
	}
	rq.lastWriteSeconds.Store(jw.runtimeSeconds.Load())
	rq.initial = r
	rq.remoteIP = remoteIP
	rq.ctx, rq.cancelFn = context.WithCancelCause(jw.BaseContext)
	if registered && r != nil {
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

// unregisterLocked cancels and unregisters a registered Request. A nil entry
// reserves its key until the Request becomes unreachable. Caller must hold
// jw.mu and rq.mu and check that jw.requests[rq.JawsKey] == rq.
func (jw *Jaws) unregisterLocked(rq *Request, err error) (cause error) {
	jawsKey := rq.JawsKey
	cause = rq.cancelLocked(err)
	jw.removePendingRequestLocked(rq)
	jw.requests[jawsKey] = nil
	jw.requestCount--
	// finishLocked captures whether the Request was claimed before detaching its
	// session, preserving the claimed WebSocket's grace period.
	rq.finishLocked()
	runtime.AddCleanup(rq, releaseRetiredRequestKey, retiredRequestKey{jw: weak.Make(jw), jawsKey: jawsKey})
	return
}

// retireNonRunningRequestLocked cancels and unregisters rq without clearing or
// pooling it. Caller must hold jw.mu; rq must not be running. A nil err cancels
// without a specific cause.
func (jw *Jaws) retireNonRunningRequestLocked(rq *Request, err error) (cause error) {
	func() {
		rq.mu.Lock()
		defer rq.mu.Unlock()
		if rq.JawsKey != 0 && jw.requests[rq.JawsKey] == rq && rq.loadState() != reqRunning {
			cause = jw.unregisterLocked(rq, err)
		}
	}()
	runtime.KeepAlive(rq)
	return
}

// recycle finishes rq and returns its reusable buffers to jw.requestBufferPool.
// The Request itself is never pooled or reused.
func (jw *Jaws) recycle(rq *Request) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	var buffers *requestBuffers
	func() {
		rq.mu.Lock()
		defer rq.mu.Unlock()
		if rq.JawsKey != 0 && jw.requests[rq.JawsKey] == rq {
			_ = jw.unregisterLocked(rq, nil)
			buffers = rq.releaseBuffersLocked()
		}
	}()
	// Return the buffers after releasing rq.mu.
	if buffers != nil {
		jw.requestBufferPool.Put(buffers)
	}
}

// cancelIfCurrent cancels rq only if it is still the [Request] registered for
// jawsKey. A caller that looks up a Request and later cancels it without holding
// jw.mu in between (the /jaws/.tail write-error path in [Jaws.ServeHTTP]) holds a
// pointer whose Request may have finished and been unregistered. Cancelling a
// finished Request is harmless (its identity is never reused), but the identity
// check avoids logging a spurious cancellation; holding jw.mu across the cancel
// keeps that check valid, since finishing requires the jw.mu write lock.
func (jw *Jaws) cancelIfCurrent(jawsKey key.Key, rq *Request, err error) {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	if jw.requests[jawsKey] == rq {
		rq.mu.Lock()
		defer rq.mu.Unlock()
		_ = jw.Log(rq.cancelLocked(err))
	}
}
