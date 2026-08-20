package jaws

// Maintainer locking notes:
//
// Core locks are acquired Jaws.mu -> Request.mu -> Session.mu. Request.muQueue and
// most per-Element widget locks are leaves. The Element state slot itself is guarded by
// Request.mu through ElementState and SetElementState. Blocking work and synchronous
// application callbacks run after releasing locks. Logging may be queued while core
// locks are held: the queue mutex is a leaf, producers never wait for delivery, and
// the worker releases it before invoking Logger.Error. Request.SetContext is the
// exception because its transform must run atomically under Request.mu and therefore
// must not block or call back into the same Request.
//
// Bound-value locks in lib/bind, lib/ui, and lib/named are released before dirtying or
// broadcasting. InitialHTMLAttrHandler callbacks run without caller-held widget or
// bound-value locks; bind.Binder acquires its own lock before calling its narrower
// InitialHTMLAttrHook. Code holding a core lock must not invoke UI value methods.
// Container reconciliation has one deliberate reverse edge: containerState.mu may be
// held while Request.NewElement takes Request.mu. Provider callbacks and validation
// precede that edge; rendering, removal, recursive cleanup, cancellation, and logging
// follow it. Core-lock holders must therefore never invoke container render or update
// methods. containerState.mu is a sync.Mutex, so the deadlock detector cannot enforce
// this rule.

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"html/template"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/secureheaders"
	"github.com/linkdata/staticserve"
)

const (
	// DefaultUpdateInterval is the default browser update interval.
	DefaultUpdateInterval = time.Millisecond * 100

	// DefaultWebSocketPingInterval is the default WebSocket read-idle interval.
	DefaultWebSocketPingInterval = time.Minute

	// DefaultWebSocketTimeout is the timeout [Jaws.Serve] passes to [Jaws.ServeWithTimeout].
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
// settings. Several are consulted on each connection or request (for example
// MaxPendingRequestsPerIP and WebSocketPingInterval), so set them all before
// exposing handlers, creating Requests, or starting [Jaws.Serve] /
// [Jaws.ServeWithTimeout]; mutating one after serving has begun is an
// unsynchronized write and is not supported. Methods document their own
// concurrency behavior and may be called concurrently when stated.
type Jaws struct {
	// CookieName is the name used for session cookies.
	//
	// It defaults to [assets.DefaultCookieName], which is derived from the
	// executable and falls back to "jaws". CookieName must be a valid, non-empty
	// HTTP cookie name; see [http.Cookie.Valid].
	CookieName  string
	AutoSession bool // Create and associate a session during a successful WebSocket upgrade when a Request has none. Defaults to false.
	// TrustForwardedHeaders enables trusted proxy header processing.
	//
	// It governs the session cookie Secure flag and WebSocket Origin scheme
	// validation through the forwarding headers recognized by
	// [secureheaders.RequestIsSecure], and the client IP used for session and
	// request binding through X-Forwarded-For and X-Real-IP. Behind a proxy that
	// terminates TLS and forwards plain HTTP, enable it and have the proxy sanitize
	// forwarding headers and set the scheme and client IP itself, or HTTPS-page
	// WebSocket upgrades are rejected. Defaults to false; enable only behind a
	// single reverse proxy you control.
	TrustForwardedHeaders bool
	Logger                Logger     // Optional logger; [Jaws.Log] dispatches Error calls asynchronously and serially
	Debug                 bool       // Enables debug HTML and WebSocket transport-error reporting. Call GenerateHeadHTML after changing it.
	MakeAuth              MakeAuthFn // Function to create ui.With.Auth for Templates. If nil, templates get the fail-open DefaultAuth (IsAdmin()==true for everyone); set it to enforce authorization. See DefaultAuth.
	// BaseContext is the parent context for Requests.
	//
	// New uses [context.Background]. If a custom context implements the optional
	// method recognized by [context.AfterFunc], that method and its returned stop
	// function must return promptly and must not synchronously call this Jaws or
	// one of its Requests, or wait for work that does. See [Request.SetContext].
	BaseContext context.Context
	// WebSocketPingInterval controls read-idle keepalive pings.
	//
	// When a WebSocket read remains pending for this interval, JaWS pings the peer.
	// Incoming data or a successful ping restarts the interval. Time spent parsing
	// or delivering already-read data does not count toward it.
	//
	// It defaults to [DefaultWebSocketPingInterval] and must be positive;
	// non-positive values do not disable probing.
	WebSocketPingInterval   time.Duration
	MaxPendingRequestsPerIP int           // Maximum number of unclaimed Requests per client IP. Defaults to DefaultMaxPendingRequestsPerIP. Set <=0 to disable the cap.
	webSocketTimeout        time.Duration // timeout duration passed to ServeWith
	maintenanceInterval     time.Duration // Serve maintenance tick interval; set by ServeWithTimeout and read under mu, zero until Serve starts
	created                 time.Time     // monotonic base captured in New(); read-only after construction, basis for runtimeSeconds
	runtimeSeconds          atomic.Int32  // whole seconds since created; refreshed during request allocation and by the Serve loop, read lock-free by MarkWritten and the eviction/idle checks
	bcastCh                 chan wire.Message
	subCh                   chan subscription
	unsubCh                 chan chan wire.Message
	updateTicker            *time.Ticker
	serving                 atomic.Bool
	loggerQueue             *loggerQueue
	defaultAuthOnce         sync.Once    // guards lazy creation of defaultAuthVal
	defaultAuthVal          *DefaultAuth // shared fail-open Auth; see [Jaws.DefaultAuth]
	requestBufferPool       sync.Pool    // reusable *requestBuffers; Requests themselves are never pooled or reused
	serveJS                 *staticserve.StaticServe
	serveCSS                *staticserve.StaticServe
	mu                      deadlock.RWMutex // protects following
	headPrefix              string
	faviconURL              string
	cspHeader               string
	tmplookers              []TemplateLookuper
	kg                      *bufio.Reader
	closeCh                 chan struct{}        // closed when Close() has been called
	requests                map[key.Key]*Request // nil entries reserve retired keys until cleanup
	requestCount            int                  // number of non-nil entries in requests
	pending                 map[netip.Addr][]*Request
	sessions                map[key.Key]*Session
	dirty                   map[any]int
	dirtOrder               int
}

// New allocates a JaWS instance with the default configuration.
//
// The returned [Jaws] value is ready for use: static assets are embedded, the
// broadcast channels and update ticker are allocated and the reusable request
// buffer pool is primed. You must still start the processing loop with [Jaws.Serve]
// or [Jaws.ServeWithTimeout] on its own goroutine before broadcasting. Call
// [Jaws.Close] when finished with the instance to free associated resources.
func New() (jw *Jaws, err error) {
	var serveJS, serveCSS *staticserve.StaticServe
	if serveJS, err = staticserve.New("/jaws/.jaws.js", []byte(assets.JavascriptText)); err == nil {
		if serveCSS, err = staticserve.New("/jaws/.jaws.css", []byte(assets.JawsCSS)); err == nil {
			tmp := &Jaws{
				CookieName:              assets.DefaultCookieName,
				BaseContext:             context.Background(),
				WebSocketPingInterval:   DefaultWebSocketPingInterval,
				MaxPendingRequestsPerIP: DefaultMaxPendingRequestsPerIP,
				webSocketTimeout:        DefaultWebSocketTimeout,
				created:                 time.Now(),
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
				jw.loggerQueue = newLoggerQueue()
				jw.requestBufferPool.New = func() any {
					return &requestBuffers{tagMap: make(map[any][]*Element)}
				}
			}
		}
	}

	return
}

// Close initiates shutdown of the [Jaws] instance.
//
// [Jaws.Done] is closed as shutdown begins. Before Close returns, the context
// returned by [Request.Context] for every current Request is canceled, including
// pending Requests whose WebSocket never connected. Non-running Requests become
// unclaimable but retain their identity while callers hold them. Active WebSocket
// handlers observe cancellation and finish asynchronously.
//
// Registered [Session] values are invalidated and detached from their Requests,
// and their key/value data is permanently cleared; new Sessions cannot be
// created after shutdown begins.
//
// Calls to [Jaws.NewRequest] after shutdown begins return Requests with
// already-canceled contexts that [Jaws.UseRequest] cannot claim. Broadcasts and
// sends may be discarded after Done closes. Close stops accepting errors from
// [Jaws.Log] and lets those already accepted drain without waiting for their
// callbacks; later Log calls are discarded. On normal return after shutdown,
// [Jaws.Serve] and [Jaws.ServeWithTimeout] wait for the drain. Subsequent calls
// to Close have no effect.
func (jw *Jaws) Close() {
	jw.mu.Lock()
	select {
	case <-jw.closeCh:
		jw.mu.Unlock()
		return
	default:
		jw.loggerQueue.close()
		close(jw.closeCh)
	}
	jw.updateTicker.Stop()
	for _, rq := range jw.requests {
		if rq == nil {
			continue
		}
		if rq.loadState() == reqRunning {
			rq.mu.Lock()
			rq.killSessionLocked(true)
			// Shutdown has no error cause. CancelCauseFunc is idempotent, so it
			// also safely handles a Request whose context is already done.
			rq.cancelFn(nil)
			rq.mu.Unlock()
		} else {
			jw.retireNonRunningRequestLocked(rq)
		}
	}
	jw.closeSessionsLocked()
	jw.mu.Unlock()
}

// Done returns a channel closed when [Jaws.Close] begins shutdown.
func (jw *Jaws) Done() <-chan struct{} {
	return jw.closeCh
}

// AddTemplateLookuper adds a [TemplateLookuper].
//
// The lookuper must be comparable so it can be removed with
// [Jaws.RemoveTemplateLookuper], and it must compare equal to itself. A value
// that is runtime-comparable yet not reflexively equal — a struct carrying a
// floating-point NaN, for example — is never matched: each add appends a new
// entry instead of deduplicating, and [Jaws.RemoveTemplateLookuper] reports
// success without removing it.
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
//
// The lookuper is matched by equality, so a value that does not compare equal to
// itself — one carrying a floating-point NaN, for example — is never found and
// this returns nil without removing it; see [Jaws.AddTemplateLookuper].
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

// RequestCounts returns the total and active Request counts.
//
// The total includes pending, claimed, and active [Request] values. It excludes
// retired Requests, even if an initial HTTP handler still holds them. The active
// count includes Requests whose [Request.ServeHTTP] loop is running.
func (jw *Jaws) RequestCounts() (total, active int) {
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	total = jw.requestCount
	for _, rq := range jw.requests {
		if rq != nil {
			if rq.loadState() == reqRunning {
				active++
			}
		}
	}
	return
}

// RequestCount returns the total Request count.
//
// It equals the total returned by [Jaws.RequestCounts].
func (jw *Jaws) RequestCount() (n int) {
	jw.mu.RLock()
	n = jw.requestCount
	jw.mu.RUnlock()
	return
}

// Log queues an error for the [Jaws.Logger] and returns err.
//
// Delivery is asynchronous and FIFO-serialized for each Jaws instance. Logger.Error
// runs without JaWS core locks and may re-enter the same Jaws subject to the normal
// lifecycle rules. A panic from Logger.Error is recovered by the logging dispatcher.
//
// Log is safe for concurrent use, including with [Jaws.Close]. It has no effect
// if jw is nil, err is nil, the Logger is nil, or shutdown has begun. It always
// returns err. The queue applies no capacity backpressure, so errors accumulate
// in memory when Logger.Error does not keep pace. Log retains err for delivery;
// callers must not mutate state exposed by err concurrently after passing it.
func (jw *Jaws) Log(err error) error {
	if err != nil && jw != nil && jw.Logger != nil {
		jw.loggerQueue.enqueue(jw.Logger, err)
	}
	return err
}

// MustLog passes a non-nil err to [Jaws.Log], or panics if no [Jaws.Logger] is
// configured.
//
// A nil err has no effect, including on a nil receiver. With a configured
// Logger, errors submitted after shutdown begins are discarded by Log.
func (jw *Jaws) MustLog(err error) {
	if err != nil {
		if jw != nil && jw.Logger != nil {
			_ = jw.Log(err)
		} else {
			panic(err)
		}
	}
}

// reportMisuse passes an API contract violation to MustLog and panics in debug
// or race builds.
func (jw *Jaws) reportMisuse(err error) {
	jw.MustLog(err)
	if deadlock.Debug {
		panic(err)
	}
}

// FaviconURL returns the favicon URL discovered by [Jaws.GenerateHeadHTML].
func (jw *Jaws) FaviconURL() (s string) {
	jw.mu.RLock()
	s = jw.faviconURL
	jw.mu.RUnlock()
	return
}

// ContentSecurityPolicy returns the Content-Security-Policy header value
// generated by [Jaws.GenerateHeadHTML].
func (jw *Jaws) ContentSecurityPolicy() (s string) {
	jw.mu.RLock()
	s = jw.cspHeader
	jw.mu.RUnlock()
	return
}

// SecureHeadersMiddleware wraps next with the JaWS security headers.
//
// It clones secureheaders.DefaultHeaders(), replacing the
// Content-Security-Policy value with [Jaws.ContentSecurityPolicy] for each
// request. The generated policy applies
// [secureheaders.ResourceDestinationAuto] to resource URLs configured by
// [Jaws.GenerateHeadHTML].
//
// Applications needing explicit destinations can instead use
// [secureheaders.Middleware] with [secureheaders.DefaultHeaders], setting its
// Content-Security-Policy with [secureheaders.BuildContentSecurityPolicy]. The
// replacement policy must include every external resource the page loads,
// including resources configured by [Jaws.GenerateHeadHTML].
//
// The returned middleware does not trust forwarded HTTPS headers. Note that the
// session cookie Secure flag is governed separately by [Jaws.TrustForwardedHeaders]
// (also false by default), so the two stay consistent unless you opt in.
func (jw *Jaws) SecureHeadersMiddleware(next http.Handler) http.Handler {
	hdrs := secureheaders.DefaultHeaders()
	delete(hdrs, "Content-Security-Policy")
	return secureHeadersMiddleware{Jaws: jw, Handler: next, Header: hdrs}
}

type secureHeadersMiddleware struct {
	*Jaws
	http.Handler
	Header http.Header
}

func (m secureHeadersMiddleware) ServeHTTP(hw http.ResponseWriter, hr *http.Request) {
	secureheaders.SetHeaders(m.Header, hw, secureheaders.RequestIsSecure(hr, false))
	hw.Header().Set("Content-Security-Policy", m.ContentSecurityPolicy())
	m.Handler.ServeHTTP(hw, hr)
}

// GenerateHeadHTML regenerates the HTML code that goes in the HEAD section.
//
// It emits the provided URL resources in extra according to
// [assets.PreloadHTML], along with the JaWS JavaScript and stylesheet. Every
// successfully parsed URL is passed to
// [secureheaders.BuildContentSecurityPolicyForURLs], and the resulting policy
// is available from [Jaws.ContentSecurityPolicy]. The favicon selected by
// [assets.PreloadHTML] is available from [Jaws.FaviconURL].
//
// A configured [Jaws.Logger] warns once for each extra URL that is omitted from
// the final markup or is absolute or scheme-relative and cannot contribute an
// explicit policy source. Warning URLs redact passwords and omit queries and
// fragments. Applications may load omitted resources manually. See
// [Jaws.SecureHeadersMiddleware] when automatic inference does not match the
// resource's request destination.
//
// Resource URLs must come from trusted application configuration because
// matched scripts are executable and CSP permissions apply to origins.
//
// If one or more URLs in extra fail to parse, GenerateHeadHTML still installs
// the regenerated head HTML and Content-Security-Policy with the failing
// resources omitted, and returns the joined parse errors.
//
// Call GenerateHeadHTML after changing [Jaws.Debug] or the extra resources.
func (jw *Jaws) GenerateHeadHTML(extra ...string) (err error) {
	var jawsurl *url.URL
	if jawsurl, err = url.Parse(jw.serveJS.Name); err == nil {
		var cssurl *url.URL
		if cssurl, err = url.Parse(jw.serveCSS.Name); err == nil {
			urls := []*url.URL{cssurl, jawsurl}
			for _, urlstr := range extra {
				if u, e := url.Parse(urlstr); e == nil {
					// Skip an extra that re-lists either built-in resource (both cssurl
					// and jawsurl were prepended above) so it is not preloaded or added
					// to the Content-Security-Policy twice. Match the complete URL: a
					// resource on another origin, or with a distinct query string, is a
					// separate resource even when it uses the same path as a built-in.
					if u.String() != jawsurl.String() && u.String() != cssurl.String() {
						urls = append(urls, u)
					}
				} else {
					err = errors.Join(err, e)
				}
			}
			headPrefix, faviconURL := assets.PreloadHTML(urls...)
			if jw.Logger != nil {
				for _, u := range urls[2:] {
					if (u.IsAbs() || u.Host != "") && secureheaders.ContentSecurityPolicySource(u) == "" {
						jw.Logger.Warn(
							"jaws: resource omitted from generated Content-Security-Policy",
							"url", resourceWarningURL(u),
						)
						continue
					}
					markup, individualFavicon := assets.PreloadHTML(u)
					if markup == "" || (individualFavicon != "" && individualFavicon != faviconURL) {
						jw.Logger.Warn("jaws: resource omitted from generated head HTML", "url", resourceWarningURL(u))
					}
				}
			}
			if jw.Debug {
				headPrefix += `<meta name="jawsDebug" content="true">`
			}
			headPrefix += `<meta name="jawsKey" content="`
			cspHeader := secureheaders.BuildContentSecurityPolicyForURLs(urls...)
			jw.mu.Lock()
			jw.headPrefix = headPrefix
			jw.faviconURL = faviconURL
			jw.cspHeader = cspHeader
			jw.mu.Unlock()
		}
	}
	return
}

func resourceWarningURL(u *url.URL) string {
	v := *u
	v.RawQuery = ""
	v.ForceQuery = false
	v.Fragment = ""
	v.RawFragment = ""
	return v.Redacted()
}

const (
	headerAllowGetHead        = http.MethodGet + ", " + http.MethodHead
	headerCacheControlNoStore = "no-store"
)

// methodAllowedGetHead reports whether r may proceed to a GET/HEAD endpoint. When
// the method is neither it writes a 405 with the Allow header and returns false.
// Checking the method per matched endpoint (rather than up front) keeps the 405 to
// genuinely matched endpoints; everything else falls through to 404.
func methodAllowedGetHead(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", headerAllowGetHead)
	w.WriteHeader(http.StatusMethodNotAllowed)
	return false
}

// ServeHTTP can handle the required JaWS endpoints, which all start with "/jaws/".
//
// The method is checked per matched endpoint, not up front: the static asset and
// .ping endpoints answer GET and HEAD (any other method gets 405 with an Allow
// header), while the per-Request key and tail-script endpoints are GET-only
// capability URLs that fall through to 404 on any other method. An unknown path or
// a wrong method on a capability URL therefore 404s rather than 405s, and never
// reveals whether a key is valid.
func (jw *Jaws) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) > 6 && strings.HasPrefix(r.URL.Path, "/jaws/") {
		if r.URL.Path[6] == '.' {
			switch r.URL.Path {
			case jw.serveCSS.Name:
				if methodAllowedGetHead(w, r) {
					jw.serveCSS.ServeHTTP(w, r)
				}
				return
			case jw.serveJS.Name:
				if methodAllowedGetHead(w, r) {
					jw.serveJS.ServeHTTP(w, r)
				}
				return
			case "/jaws/.ping":
				if methodAllowedGetHead(w, r) {
					w.Header().Set("Cache-Control", headerCacheControlNoStore)
					select {
					case <-jw.Done():
						w.WriteHeader(http.StatusServiceUnavailable)
					default:
						w.WriteHeader(http.StatusNoContent)
					}
				}
				return
			default:
				if r.Method == http.MethodGet && jw.serveTailScript(w, r) {
					return
				}
			}
		} else if r.Method == http.MethodGet {
			// A path here addresses a specific Request by its key, a 64-bit CSPRNG
			// value that appears only in the page's <meta name="jawsKey"> (read by
			// jaws.js to build the WebSocket URL). It is in no href/src a crawler would
			// follow and guessing it is 1 in 2^63, so whoever reaches this branch knows
			// the key and is the client connecting its WebSocket (or fetching the
			// /noscript fallback). We therefore do not special-case non-WebSocket,
			// prefetch or probe traffic: UseRequest claims the single-use Request and
			// Request.ServeHTTP validates the Origin (cross-site WebSocket hijack
			// defense) before upgrading. Consuming the key on a non-handshake request is
			// acceptable because only a holder of the key can reach here.
			jawsKey, tail := key.Parse(r.URL.Path[6:])
			if jawsKey != 0 && (tail == "" || tail == "/noscript") {
				if rq := jw.UseRequest(jawsKey, r); rq != nil {
					rq.ServeHTTP(w, r)
					return
				}
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
