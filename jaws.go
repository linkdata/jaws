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
// the event goroutine. [Element.JawsRender] and [Element.Freeze] publish the final
// handler slice through the Element's atomic frozen flag; request event dispatch
// reads handlers only after observing that flag. This also covers child Elements
// rendered after a WebSocket connects: a preemptive event for a still-rendering
// Element is ignored. Handlers must not be added after JawsRender returns or Freeze
// is called. All builds enforce this through an internal chokepoint that drops late
// additions; debug builds panic instead.
//
// # Testing
//
// Always run the tests with the -race flag. Race builds set deadlock.Debug and
// deadlock.Enabled, exercising the deadlock lock-order detector described above
// and JaWS debug-gated runtime checks such as the late-handler panic. If the race
// detector is unavailable, use -tags "debug deadlock" so both categories stay
// active: the debug tag sets deadlock.Debug, while the deadlock tag enables the
// detector. Those JaWS debug branches are compile-time dead in normal builds, so
// a plain "go test" neither exercises them nor reports their statement coverage.
// Runtime tag-comparability checks in [github.com/linkdata/jaws/lib/tag] run in
// every build. CI builds with -race.
package jaws

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
// settings. Several are consulted on each connection or request (for example
// MaxPendingRequestsPerIP and WebSocketPingInterval), so set them all before
// exposing handlers, creating Requests, or starting [Jaws.Serve] /
// [Jaws.ServeWithTimeout]; mutating one after serving has begun is an
// unsynchronized write and is not supported. Methods document their own
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
	created                 time.Time       // monotonic base captured in New(); read-only after construction, basis for runtimeNanos
	runtimeNanos            atomic.Int64    // nanoseconds since created; refreshed during request allocation and by the Serve loop, read lock-free by MarkWritten and the eviction/idle checks
	bcastCh                 chan wire.Message
	subCh                   chan subscription
	unsubCh                 chan chan wire.Message
	updateTicker            *time.Ticker
	serving                 atomic.Bool
	defaultAuthOnce         sync.Once    // guards lazy creation of defaultAuthVal
	defaultAuthVal          *DefaultAuth // shared fail-open Auth; see [Jaws.DefaultAuth]
	requestBufferPool       sync.Pool
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
// buffer pool is primed. You must still start the processing loop with
// [Jaws.Serve] or [Jaws.ServeWithTimeout] on its own goroutine before
// broadcasting. Call [Jaws.Close] when finished with the instance to free
// associated resources.
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
// Calls to [Jaws.NewRequest] after shutdown begins return Requests with
// already-canceled contexts that [Jaws.UseRequest] cannot claim. Broadcasts and
// sends may be discarded after Done closes. Subsequent calls to Close have no
// effect.
func (jw *Jaws) Close() {
	jw.mu.Lock()
	select {
	case <-jw.closeCh:
		jw.mu.Unlock()
		return
	default:
		close(jw.closeCh)
	}
	jw.updateTicker.Stop()
	for _, rq := range jw.requests {
		if rq == nil {
			continue
		}
		if rq.running.Load() {
			rq.mu.Lock()
			// Shutdown has no error cause. CancelCauseFunc is idempotent, so it
			// also safely handles a Request whose context is already done.
			rq.cancelFn(nil)
			rq.mu.Unlock()
		} else {
			jw.retireNonRunningRequestLocked(rq)
		}
	}
	jw.mu.Unlock()
}

// Done returns a channel closed when [Jaws.Close] begins shutdown.
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
			if rq.running.Load() {
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
// Content-Security-Policy value with [Jaws.ContentSecurityPolicy] for each
// request so responses allow the resources configured by [Jaws.GenerateHeadHTML].
//
// The returned middleware does not trust forwarded HTTPS headers. Note that the
// session cookie Secure flag is governed separately by [Jaws.TrustForwardedHeaders]
// (also false by default), so the two stay consistent unless you opt in.
// The next handler must be non-nil.
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
	hw.Header()["Content-Security-Policy"] = []string{m.ContentSecurityPolicy()}
	m.Handler.ServeHTTP(hw, hr)
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

var (
	headerAllowGetHead        = []string{http.MethodGet + ", " + http.MethodHead}
	headerCacheControlNoStore = []string{"no-store"}
)

// methodAllowedGetHead reports whether r may proceed to a GET/HEAD endpoint. When
// the method is neither it writes a 405 with the Allow header and returns false.
// Checking the method per matched endpoint (rather than up front) keeps the 405 to
// genuinely matched endpoints; everything else falls through to 404.
func methodAllowedGetHead(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header()["Allow"] = headerAllowGetHead
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
					w.Header()["Cache-Control"] = headerCacheControlNoStore
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
