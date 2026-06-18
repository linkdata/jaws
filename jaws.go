// Package jaws provides a mechanism to create dynamic
// webpages using JavaScript and WebSockets.
//
// It integrates well with Go's [html/template] package,
// but can be used without it. It can be used with any
// router that supports the standard [http.Handler] interface.
//
// This package holds the core engine and the [UI] interfaces. The standard
// widgets (Span, Button, Select, Text, and so on) and the RequestWriter helper
// methods live in [github.com/linkdata/jaws/lib/ui], and value binding lives in
// [github.com/linkdata/jaws/lib/bind].
//
// # Locking
//
// The package uses a single, acyclic lock hierarchy. When more than one of these
// locks is held at once they must be acquired in this order, outermost first:
//
//	Jaws.mu  ->  Request.mu  ->  Session.mu
//
// Request.muQueue and per-[Element] state are leaf locks taken below all of the
// above. Blocking work (channel sends, user callbacks) is always performed after
// snapshotting the needed state and releasing the relevant lock; see
// [Session.Broadcast] and [Session.Close] for the canonical pattern. The
// [Request.SetContext] transform is the deliberate exception: it runs while
// holding Request.mu so the read-modify-write is atomic, and therefore must not
// call back into the same Request or block.
//
// UI value and widget types in the subpackages carry their own leaf locks that
// guard the bound value: the binders in [github.com/linkdata/jaws/lib/bind], the
// JsVar in [github.com/linkdata/jaws/lib/ui] and the named values in
// [github.com/linkdata/jaws/lib/named]. These are leaves with respect to each
// other, acquired containing-before-contained (for example a named BoolArray's
// mutex is taken before a member Bool's). They sit strictly below the three core
// locks: every value type mutates the bound value under its value lock, releases
// it, and only then marks the [Element] dirty or broadcasts the change (which
// ultimately takes the outermost Jaws.mu), so a value lock is never held while a
// core lock is acquired. lib/bind, lib/ui and lib/named all follow this
// mutate-release-then-dirty pattern, and new value types must too. The safety of
// this rests on an invariant the deadlock detector cannot enforce (value locks are
// leaves distinct from Jaws.mu): no code path holding Jaws.mu, Request.mu or
// Session.mu ever calls into a UI value's Get/Set/Dirty methods, which are the only
// callers that take a value lock — were it otherwise, the later dirty step's Jaws.mu
// acquisition would invert the core lock order. Code holding any of the three core
// locks must therefore never invoke a UI value method.
//
// A deliberate reverse edge lives in [github.com/linkdata/jaws/lib/ui]:
// ContainerHelper.reconcile holds its own widget mutex while calling
// [Request.NewElement], which takes Request.mu — again leaf-before-core. It is safe
// for the same reason: no code path holding any of the three core locks ever
// invokes a widget's render or update method (the Serve loop calls JawsRender and
// JawsUpdate only after releasing Request.mu). Code holding Jaws.mu, Request.mu or
// Session.mu must therefore never call a container's render/update entry points.
// Note the widget mutex is a plain sync.Mutex, so deadlock.Debug cannot observe
// this inversion; the invariant is maintained by convention.
//
// [Element] handlers are an intentional exception to the locking rules: they are
// populated only while an Element is rendered and are then read without a lock on
// the event goroutine. This is safe solely because rendering completes before any
// event for that Element can be processed, so handlers must not be added after
// [Element.JawsRender] returns (or after [Element.Freeze] for update-only
// registrations). All builds enforce this: every handler mutator funnels through
// an internal chokepoint guarded by a lockless atomic flag that drops late
// additions; debug builds panic instead.
//
// # Testing
//
// Always run the tests with the -race flag (or -tags debug). Both set
// deadlock.Debug, which enables the debug-gated runtime invariant checks: the
// lock-order verification described above and the late-handler panic. Those
// branches are compile-time dead in normal builds, so a plain "go test" neither
// exercises them nor reports their statement coverage. Runtime tag-comparability
// checks in [github.com/linkdata/jaws/lib/tag] run in every build. CI builds with
// -tags debug -race.
package jaws

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/secureheaders"
	"github.com/linkdata/staticserve"
)

const (
	// DefaultUpdateInterval is the default browser update interval.
	DefaultUpdateInterval = time.Millisecond * 100

	// DefaultWebSocketPingInterval is the default WebSocket keepalive ping interval.
	DefaultWebSocketPingInterval = time.Minute

	// DefaultWebSocketTimeout is the default time allowed for WebSocket connect and ping responses.
	DefaultWebSocketTimeout = time.Second * 10

	// DefaultMaxPendingRequestsPerIP is the default maximum number of unclaimed
	// Requests allowed for each client IP.
	DefaultMaxPendingRequestsPerIP = 100
)

type subscription struct {
	msgCh chan wire.Message
	rq    *Request
}

// Jid is the identifier type used for HTML elements managed by JaWS.
//
// It is provided as a convenience alias to the value defined in the jid
// subpackage so applications do not have to import that package directly
// when working with element IDs.
type Jid = jid.Jid // convenience alias

// Jaws holds the server-side state and configuration for a JaWS instance.
//
// A single Jaws value coordinates template lookup, session handling and the
// request lifecycle that keeps the browser and backend synchronized via
// WebSockets. The zero value is not ready for use; construct instances with
// [New] to ensure the helper goroutines and static assets are prepared.
//
// The exported configuration fields are ordinary fields, not live synchronized
// settings. Set them before exposing handlers, creating Requests, or starting
// [Jaws.Serve] / [Jaws.ServeWithTimeout]. Methods document their own
// concurrency behavior and may be called concurrently when stated.
type Jaws struct {
	CookieName              string          // Name for session cookies; defaults to a name derived from the executable ([assets.DefaultCookieName]), falling back to "jaws"
	AutoSession             bool            // Create a session during WebSocket upgrade when a Request has none. Defaults to false.
	TrustForwardedHeaders   bool            // Trust X-Forwarded-* headers: governs the session cookie Secure flag (X-Forwarded-Proto) and the client IP used for session/request binding (X-Forwarded-For/X-Real-IP). Defaults to false; only enable behind a single reverse proxy you control that sets these headers.
	Logger                  Logger          // Optional logger to use
	Debug                   bool            // Set to true to enable debug info in generated HTML code. Call GenerateHeadHTML after changing it.
	MakeAuth                MakeAuthFn      // Function to create ui.With.Auth for Templates. If nil, templates get the fail-open DefaultAuth (IsAdmin()==true for everyone); set it to enforce authorization. See DefaultAuth.
	BaseContext             context.Context // Non-nil base context for Requests, set to context.Background() in New()
	WebSocketPingInterval   time.Duration   // Interval between keepalive pings on active WebSocket connections. Defaults to DefaultWebSocketPingInterval. Set <=0 to disable keepalive pings.
	MaxPendingRequestsPerIP int             // Maximum number of unclaimed Requests per client IP. Defaults to DefaultMaxPendingRequestsPerIP. Set <=0 to disable the cap.
	webSocketTimeout        time.Duration   // timeout duration passed to ServeWith
	maintenanceInterval     time.Duration   // Serve maintenance tick interval; set by ServeWithTimeout and read under mu, zero until Serve starts
	bcastCh                 chan wire.Message
	subCh                   chan subscription
	unsubCh                 chan chan wire.Message
	updateTicker            *time.Ticker
	serving                 atomic.Bool
	defaultAuthOnce         sync.Once    // guards lazy creation of defaultAuthVal
	defaultAuthVal          *DefaultAuth // shared fail-open Auth; see [Jaws.DefaultAuth]
	reqPool                 sync.Pool
	serveJS                 *staticserve.StaticServe
	serveCSS                *staticserve.StaticServe
	mu                      deadlock.RWMutex // protects following
	headPrefix              string
	faviconURL              string
	cspHeader               string
	tmplookers              []TemplateLookuper
	kg                      *bufio.Reader
	closeCh                 chan struct{} // closed when Close() has been called
	requests                map[key.Key]*Request
	pending                 map[netip.Addr][]*Request
	sessions                map[key.Key]*Session
	dirty                   map[any]int
	dirtOrder               int
}

// New allocates a JaWS instance with the default configuration.
//
// The returned [Jaws] value is ready for use: static assets are embedded, the
// broadcast channels and update ticker are allocated and the request pool is
// primed. You must still start the processing loop with [Jaws.Serve] or
// [Jaws.ServeWithTimeout] on its own goroutine before broadcasting. Call
// [Jaws.Close] when finished with the instance to free associated resources.
func New() (jw *Jaws, err error) {
	var serveJS, serveCSS *staticserve.StaticServe
	if serveJS, err = staticserve.New("/jaws/.jaws.js", assets.JavascriptText); err == nil {
		if serveCSS, err = staticserve.New("/jaws/.jaws.css", assets.JawsCSS); err == nil {
			tmp := &Jaws{
				CookieName:              assets.DefaultCookieName,
				BaseContext:             context.Background(),
				WebSocketPingInterval:   DefaultWebSocketPingInterval,
				MaxPendingRequestsPerIP: DefaultMaxPendingRequestsPerIP,
				webSocketTimeout:        DefaultWebSocketTimeout,
				serveJS:                 serveJS,
				serveCSS:                serveCSS,
				bcastCh:                 make(chan wire.Message, 1),
				subCh:                   make(chan subscription),
				unsubCh:                 make(chan chan wire.Message),
				updateTicker:            time.NewTicker(DefaultUpdateInterval),
				kg:                      bufio.NewReader(rand.Reader),
				requests:                make(map[key.Key]*Request),
				pending:                 make(map[netip.Addr][]*Request),
				sessions:                make(map[key.Key]*Session),
				dirty:                   make(map[any]int),
				closeCh:                 make(chan struct{}),
			}
			if err = tmp.GenerateHeadHTML(); err == nil {
				jw = tmp
				jw.reqPool.New = func() any {
					return (&Request{
						Jaws:   jw,
						tagMap: make(map[any][]*Element),
					}).clearLocked()
				}
			}
		}
	}

	return
}

// Close frees resources associated with the [Jaws] object, and
// closes the completion channel if the [Jaws] was created with [New].
// Once the completion channel is closed, broadcasts and sends may be discarded.
// Subsequent calls to Close() have no effect.
func (jw *Jaws) Close() {
	jw.mu.Lock()
	select {
	case <-jw.closeCh:
		// already closed
	default:
		close(jw.closeCh)
	}
	jw.updateTicker.Stop()
	jw.mu.Unlock()
}

// Done returns the channel that is closed when [Jaws.Close] has been called.
func (jw *Jaws) Done() <-chan struct{} {
	return jw.closeCh
}

// AddTemplateLookuper adds a [TemplateLookuper].
//
// The lookuper must be comparable so it can be removed with
// [Jaws.RemoveTemplateLookuper].
func (jw *Jaws) AddTemplateLookuper(tl TemplateLookuper) (err error) {
	if tl != nil {
		if err = tag.NewErrNotComparable(tl); err == nil {
			jw.mu.Lock()
			if !slices.Contains(jw.tmplookers, tl) {
				jw.tmplookers = append(jw.tmplookers, tl)
			}
			jw.mu.Unlock()
		}
	}
	return
}

// RemoveTemplateLookuper removes the given [TemplateLookuper].
func (jw *Jaws) RemoveTemplateLookuper(tl TemplateLookuper) (err error) {
	if tl != nil {
		if err = tag.NewErrNotComparable(tl); err == nil {
			jw.mu.Lock()
			jw.tmplookers = slices.DeleteFunc(jw.tmplookers, func(x TemplateLookuper) bool { return x == tl })
			jw.mu.Unlock()
		}
	}
	return
}

// LookupTemplate queries the known [TemplateLookuper] values in the order
// they were added and returns the first found.
func (jw *Jaws) LookupTemplate(name string) *template.Template {
	jw.mu.RLock()
	tmplookers := slices.Clone(jw.tmplookers)
	jw.mu.RUnlock()
	for _, tl := range tmplookers {
		if t := tl.Lookup(name); t != nil {
			return t
		}
	}
	return nil
}

// RequestCounts returns the number of [Request] values.
//
// The total count includes all Requests, including those being rendered,
// those waiting for the WebSocket callback and those active. The active count
// includes Requests whose WebSocket [Request.ServeHTTP] loop is running.
func (jw *Jaws) RequestCounts() (total, active int) {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	total = len(jw.requests)
	for _, rq := range jw.requests {
		if rq.running.Load() {
			active++
		}
	}
	return
}

// RequestCount returns the total number of [Request] values, equal to the total
// returned by [Jaws.RequestCounts] (see it for what the count includes).
func (jw *Jaws) RequestCount() (n int) {
	n, _ = jw.RequestCounts()
	return
}

// Log sends an error to the [Jaws.Logger] if set.
// Has no effect if err is nil or the Logger is nil.
// Returns err.
func (jw *Jaws) Log(err error) error {
	if err != nil && jw != nil && jw.Logger != nil {
		jw.Logger.Error("jaws", "err", err)
	}
	return err
}

// MustLog sends an error to the [Jaws.Logger] if set, or
// panics with the given error if the Logger is nil.
// Has no effect if err is nil.
//
// Some update-time paths cannot return errors to their caller and report them
// through MustLog. Set [Jaws.Logger] when those errors should be logged instead
// of treated as fatal programming errors.
func (jw *Jaws) MustLog(err error) {
	if err != nil {
		if jw != nil && jw.Logger != nil {
			jw.Logger.Error("jaws", "err", err)
		} else {
			panic(err)
		}
	}
}

// reportMisuse reports a violated API contract (a programming error).
//
// It reports err through [Jaws.MustLog] (which logs it, or panics if no Logger is
// set) and, in debug builds, additionally panics to fail fast. So in production
// with a Logger configured the mistake is logged and the caller continues without
// applying the offending operation, while debug builds and unconfigured servers
// still stop on it.
func (jw *Jaws) reportMisuse(err error) {
	jw.MustLog(err)
	if deadlock.Debug {
		panic(err)
	}
}

var nextID atomic.Int64

// NextID returns an int64 unique within the lifetime of the program.
func NextID() int64 {
	return nextID.Add(1)
}

// AppendID appends the result of NextID() in text form to the given slice.
func AppendID(b []byte) []byte {
	return strconv.AppendInt(b, NextID(), 32)
}

// MakeID returns a string in the form "jaws.X" where X is unique within the lifetime of the program.
func MakeID() string {
	return string(AppendID([]byte("jaws.")))
}

// NewRequest returns a new pending JaWS request.
//
// Call this as soon as you start processing an HTML request, and store the
// returned [Request] pointer so it can be used while constructing the HTML
// response in order to register the JaWS IDs you use in the response, and
// use its [Request.JawsKey] when sending the JavaScript portion of the reply.
//
// Automatic timeout handling is performed by [Jaws.ServeWithTimeout]. The default
// [Jaws.Serve] helper uses a 10-second timeout.
//
// It panics if the system CSPRNG ([crypto/rand]) fails while generating the request
// key, which does not happen on supported platforms.
func (jw *Jaws) NewRequest(r *http.Request) (rq *Request) {
	remoteIP := jw.clientIP(r)

	var toLog []error
	func() {
		jw.mu.Lock()
		defer jw.mu.Unlock()
		toLog = jw.limitPendingRequestsLocked(remoteIP)
		for rq == nil {
			jawsKey := jw.nonZeroRandomLocked()
			if _, ok := jw.requests[jawsKey]; !ok {
				rq = jw.getRequestLocked(jawsKey, r, remoteIP)
				jw.requests[jawsKey] = rq
				jw.pending[rq.remoteIP] = append(jw.pending[rq.remoteIP], rq)
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

// limitPendingRequestsLocked evicts pending Requests for remoteIP until the cap is
// satisfied, returning the eviction causes for the caller to log after releasing
// jw.mu (see the package locking contract). Caller must hold jw.mu.
func (jw *Jaws) limitPendingRequestsLocked(remoteIP netip.Addr) (toLog []error) {
	limit := jw.MaxPendingRequestsPerIP
	if limit > 0 {
		now := time.Now()
		for len(jw.pending[remoteIP]) >= limit {
			victim := jw.oldestEvictablePendingLocked(remoteIP, now)
			if victim == nil {
				// Every pending Request for this IP is rendering or rendered too
				// recently to evict safely. Recycling a still-rendering one would
				// recycle a Request whose initial HTML is still being assembled on an
				// HTTP goroutine that holds no jw.mu, letting a later NewRequest reuse
				// the pooled pointer under a new key while that goroutine keeps
				// appending elements (contaminating the new Request and leaking its
				// key). Prefer a brief, self-correcting overshoot of the cap: the
				// renders finish and connect (or time out and get recycled by the
				// maintenance pass) shortly.
				return
			}
			if cause := jw.recycleLockedWithCause(victim, newErrTooManyPendingRequests(remoteIP, limit)); cause != nil {
				toLog = append(toLog, cause)
			}
		}
	}
	return
}

// oldestEvictablePendingLocked returns the oldest pending [Request] for remoteIP
// that is safe to recycle, or nil if every one of them might still be rendering.
// now is the reference time for the recency check; the caller passes a single value
// so all candidates are judged against the same instant. Caller must hold jw.mu.
//
// A Request is spared when its Rendering flag is set (it has been written to by
// [RequestWriter] and may still be assembling its initial HTML, which recycling would
// corrupt — see [Jaws.limitPendingRequestsLocked]), OR when it last rendered within
// 2*maintenanceInterval. The recency check is required because [Request.maintenance]
// clears Rendering once per maintenance interval: between that clear and the render
// goroutine's next write the flag reads false even though the HTML is still in flight,
// so keying eviction on the flag alone would recycle a live render. lastWrite is
// refreshed by that same maintenance pass while rendering is active, so a Request that
// wrote within the last two intervals is treated as possibly-rendering and spared,
// while one idle longer than that has finished and is evictable. Reading lastWrite
// takes rq.mu beneath jw.mu, the order documented in the package "Locking" section.
//
// maintenanceInterval is zero until [Jaws.ServeWithTimeout] starts. With no maintenance
// pass running nothing clears Rendering, so the flag alone is authoritative and the
// recency window is correctly skipped.
func (jw *Jaws) oldestEvictablePendingLocked(remoteIP netip.Addr, now time.Time) *Request {
	for _, rq := range jw.pending[remoteIP] {
		if rq.Rendering.Load() {
			continue
		}
		if jw.maintenanceInterval > 0 {
			rq.mu.RLock()
			recentlyRendered := now.Sub(rq.lastWrite) <= 2*jw.maintenanceInterval
			rq.mu.RUnlock()
			if recentlyRendered {
				continue
			}
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
func (jw *Jaws) UseRequest(jawsKey key.Key, r *http.Request) (rq *Request) {
	if jawsKey != 0 {
		var err error
		jw.mu.Lock()
		if waitingRq, ok := jw.requests[jawsKey]; ok {
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

// FaviconURL returns the favicon URL discovered by [Jaws.GenerateHeadHTML].
func (jw *Jaws) FaviconURL() (s string) {
	jw.mu.RLock()
	s = jw.faviconURL
	jw.mu.RUnlock()
	return
}

// ContentSecurityPolicy returns the generated Content-Security-Policy header value.
func (jw *Jaws) ContentSecurityPolicy() (s string) {
	jw.mu.RLock()
	s = jw.cspHeader
	jw.mu.RUnlock()
	return
}

// SecureHeadersMiddleware wraps next with security headers that match the
// current JaWS configuration.
//
// It clones secureheaders.DefaultHeaders(), replacing the
// Content-Security-Policy value with [Jaws.ContentSecurityPolicy] so responses allow
// the resources configured by [Jaws.GenerateHeadHTML].
//
// The returned middleware does not trust forwarded HTTPS headers. Note that the
// session cookie Secure flag is governed separately by [Jaws.TrustForwardedHeaders]
// (also false by default), so the two stay consistent unless you opt in.
// The next handler must be non-nil.
func (jw *Jaws) SecureHeadersMiddleware(next http.Handler) http.Handler {
	hdrs := secureheaders.DefaultHeaders()
	hdrs["Content-Security-Policy"] = []string{jw.ContentSecurityPolicy()}
	return secureheaders.Middleware{
		Handler: next,
		Header:  hdrs,
	}
}

// GenerateHeadHTML regenerates the HTML code that goes in the HEAD section,
// ensuring that the provided URL resources in extra are loaded, along with the
// JaWS JavaScript.
//
// If one of the resources is named "favicon", its URL will be stored and can
// be retrieved using [Jaws.FaviconURL].
//
// If one or more URLs in extra fail to parse, GenerateHeadHTML still installs
// the regenerated head HTML and Content-Security-Policy with the failing
// resources omitted, and returns the joined parse errors.
//
// You only need to call this if you add your own images, scripts and stylesheets.
func (jw *Jaws) GenerateHeadHTML(extra ...string) (err error) {
	var jawsurl *url.URL
	if jawsurl, err = url.Parse(jw.serveJS.Name); err == nil {
		var cssurl *url.URL
		if cssurl, err = url.Parse(jw.serveCSS.Name); err == nil {
			var urls []*url.URL
			urls = append(urls, cssurl)
			urls = append(urls, jawsurl)
			for _, urlstr := range extra {
				if u, e := url.Parse(urlstr); e == nil {
					// Skip an extra that re-lists either built-in resource (both cssurl
					// and jawsurl were prepended above) so it is not preloaded or added
					// to the Content-Security-Policy twice.
					if !strings.HasSuffix(u.Path, jawsurl.Path) && !strings.HasSuffix(u.Path, cssurl.Path) {
						urls = append(urls, u)
					}
				} else {
					err = errors.Join(err, e)
				}
			}
			headPrefix, faviconURL := assets.PreloadHTML(urls...)
			if jw.Debug {
				headPrefix += `<meta name="jawsDebug" content="true">`
			}
			headPrefix += `<meta name="jawsKey" content="`
			cspHeader := secureheaders.BuildContentSecurityPolicy(urls)
			jw.mu.Lock()
			jw.headPrefix = headPrefix
			jw.faviconURL = faviconURL
			jw.cspHeader = cspHeader
			jw.mu.Unlock()
		}
	}
	return
}

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
	now := time.Now()
	for _, rq := range jw.requests {
		if expired, cause := rq.maintenance(now, requestTimeout); expired {
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

// equalIP reports whether a and b identify the same client for the purpose of
// session and request-key binding. Two loopback addresses always compare equal
// so that a reverse proxy connecting to the backend over loopback does not break
// binding; the consequence is that when every request arrives from loopback (the
// typical proxied deployment without forwarded-IP binding) IP binding is a no-op.
// Enable [Jaws.TrustForwardedHeaders] to bind on the forwarded client IP instead
// (see the clientIP method).
func equalIP(a, b netip.Addr) bool {
	return a.Compare(b) == 0 || (a.IsLoopback() && b.IsLoopback())
}

func parseIP(remoteAddr string) (ip netip.Addr) {
	if remoteAddr != "" {
		if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
			ip, _ = netip.ParseAddr(host)
		} else {
			ip, _ = netip.ParseAddr(remoteAddr)
		}
	}
	return
}

// clientIP returns the address used to bind sessions and request keys to a
// client. When [Jaws.TrustForwardedHeaders] is set it prefers the client IP from
// the proxy-supplied forwarded headers, so binding keeps working behind a reverse
// proxy that connects over loopback; otherwise (and as a fallback) it uses the
// transport peer address. TrustForwardedHeaders must only be enabled behind a
// single reverse proxy you control that sets these headers (see the field doc).
func (jw *Jaws) clientIP(r *http.Request) (ip netip.Addr) {
	if r != nil {
		if jw.TrustForwardedHeaders {
			if fip, ok := forwardedClientIP(r.Header); ok {
				return fip
			}
		}
		ip = parseIP(r.RemoteAddr)
	}
	return
}

// forwardedClientIP extracts the client IP from proxy-supplied headers. It uses
// the leftmost X-Forwarded-For entry (the original client as seen by a single
// trusted proxy), falling back to X-Real-IP. Callers must only trust these
// headers when behind a controlled proxy (see [Jaws.TrustForwardedHeaders]).
func forwardedClientIP(h http.Header) (netip.Addr, bool) {
	if xff := h.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if ip, err := netip.ParseAddr(textproto.TrimString(first)); err == nil {
			return ip, true
		}
	}
	if xrip := textproto.TrimString(h.Get("X-Real-Ip")); xrip != "" {
		if ip, err := netip.ParseAddr(xrip); err == nil {
			return ip, true
		}
	}
	return netip.Addr{}, false
}

// getRequestLocked allocates a Request from the pool for jawsKey. remoteIP is the
// already-resolved client IP for r (see NewRequest, the sole caller), passed in to
// avoid recomputing jw.clientIP(r). Caller must hold jw.mu.
func (jw *Jaws) getRequestLocked(jawsKey key.Key, r *http.Request, remoteIP netip.Addr) (rq *Request) {
	rq = jw.reqPool.Get().(*Request)
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rq.JawsKey = jawsKey
	rq.lastWrite = time.Now()
	rq.initial = r
	rq.remoteIP = remoteIP
	rq.ctx, rq.cancelFn = context.WithCancelCause(jw.BaseContext)
	if r != nil {
		if sess := jw.getSessionLocked(getCookieSessionsIDs(r.Header, jw.CookieName), rq.remoteIP); sess != nil {
			sess.addRequest(rq)
			rq.session = sess
		}
	}
	return rq
}

// recycleLockedWithCause recycles rq, optionally cancelling its context with err.
// It returns the cancellation cause (or nil) instead of logging it, so the caller
// can log it after releasing jw.mu (see the package locking contract). Caller must
// hold jw.mu.
func (jw *Jaws) recycleLockedWithCause(rq *Request, err error) (cause error) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if rq.JawsKey != 0 {
		if err != nil {
			cause = rq.cancelLocked(err)
		}
		jw.removePendingRequestLocked(rq)
		delete(jw.requests, rq.JawsKey)
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

var headerCacheControlNoStore = []string{"no-store"}

// ServeHTTP can handle the required JaWS endpoints, which all start with "/jaws/".
func (jw *Jaws) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if len(r.URL.Path) > 6 && strings.HasPrefix(r.URL.Path, "/jaws/") {
		if r.URL.Path[6] == '.' {
			switch r.URL.Path {
			case jw.serveCSS.Name:
				jw.serveCSS.ServeHTTP(w, r)
				return
			case jw.serveJS.Name:
				jw.serveJS.ServeHTTP(w, r)
				return
			case "/jaws/.ping":
				w.Header()["Cache-Control"] = headerCacheControlNoStore
				select {
				case <-jw.Done():
					w.WriteHeader(http.StatusServiceUnavailable)
				default:
					w.WriteHeader(http.StatusNoContent)
				}
				return
			default:
				if jawsKeyString, ok := strings.CutPrefix(r.URL.Path, "/jaws/.tail/"); ok {
					jawsKey := key.Parse(jawsKeyString)
					remoteIP := jw.clientIP(r)
					// Hold jw.mu (read) across both the lookup and the drain: recycling
					// needs the jw.mu write lock, so rq cannot be recycled and reused
					// under a different key while we drain its queue. A stale key either
					// misses the map (404) or drains its own genuine content. The network
					// write is done after releasing jw.mu so a slow client cannot stall
					// recycling or the Serve loop.
					jw.mu.RLock()
					rq := jw.requests[jawsKey]
					// Bind the tail fetch to the client like the WebSocket claim path
					// (Request.claim): the one-shot tail is drained only when the fetch
					// comes from the same client IP that the initial request was issued
					// to (loopback-aware, see equalIP). rq.remoteIP is stable here because
					// recycling requires the jw.mu write lock. A mismatch is treated as not
					// found, so a leaked key cannot drain (and thereby deny) another
					// client's tail. The WebSocket carries all live data, so this only
					// closes the cross-IP read of the already-rendered attribute/class
					// fragments and the cross-IP one-shot race.
					if rq != nil && !equalIP(remoteIP, rq.remoteIP) {
						rq = nil
					}
					var b []byte
					var sent bool
					if rq != nil {
						b, sent = rq.drainTailScript()
					}
					jw.mu.RUnlock()
					if rq != nil {
						if err := rq.writeTailResponse(w, b, sent); err != nil {
							jw.cancelIfCurrent(jawsKey, rq, err)
						}
						return
					}
				}
			}
		} else if rq := jw.UseRequest(key.Parse(r.URL.Path[6:]), r); rq != nil {
			rq.ServeHTTP(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
