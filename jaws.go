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
// [Session.Broadcast] and [Session.Close] for the canonical pattern.
//
// UI value and widget types in the subpackages carry their own leaf locks that
// guard the bound value: the binders in [github.com/linkdata/jaws/lib/bind], the
// JsVar in [github.com/linkdata/jaws/lib/ui] and the named values in
// [github.com/linkdata/jaws/lib/named]. These are leaves with respect to each
// other, acquired containing-before-contained (for example a named BoolArray's
// mutex is taken before a member Bool's). They sit below the three core locks
// above, but with one deliberate reverse edge: marking an [Element] dirty or
// broadcasting a change ultimately takes the outermost Jaws.mu, and a value type
// may run that side effect while still holding its own value lock (lib/named does
// this; lib/bind and lib/ui release the value lock first). That is the reverse of
// the "outermost first" rule and is safe only because the edge never closes a
// cycle: no code path holding Jaws.mu, Request.mu or Session.mu ever calls into a
// UI value's Get/Set/Dirty methods, which are the only callers that take a value
// lock. Code holding any of the three core locks must therefore never invoke a UI
// value method. New value types should prefer the lib/bind / lib/ui pattern
// (mutate under the value lock, release it, then mark dirty) so they do not depend
// on this invariant.
//
// A second deliberate reverse edge lives in [github.com/linkdata/jaws/lib/ui]:
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
package jaws

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
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
	CookieName              string          // Name for session cookies, defaults to "jaws"
	AutoSession             bool            // Create a session during WebSocket upgrade when a Request has none. Defaults to false.
	TrustForwardedHeaders   bool            // Trust X-Forwarded-* headers: governs the session cookie Secure flag (X-Forwarded-Proto) and the client IP used for session/request binding (X-Forwarded-For/X-Real-IP). Defaults to false; only enable behind a single reverse proxy you control that sets these headers.
	Logger                  Logger          // Optional logger to use
	Debug                   bool            // Set to true to enable debug info in generated HTML code. Call GenerateHeadHTML after changing it.
	MakeAuth                MakeAuthFn      // Function to create ui.With.Auth for Templates. If nil, templates get the fail-open DefaultAuth (IsAdmin()==true for everyone); set it to enforce authorization. See DefaultAuth.
	BaseContext             context.Context // Non-nil base context for Requests, set to context.Background() in New()
	WebSocketPingInterval   time.Duration   // Interval between keepalive pings on active WebSocket connections. Defaults to DefaultWebSocketPingInterval. Set <=0 to disable keepalive pings.
	MaxPendingRequestsPerIP int             // Maximum number of unclaimed Requests per client IP. Defaults to DefaultMaxPendingRequestsPerIP. Set <=0 to disable the cap.
	webSocketTimeout        time.Duration   // timeout duration passed to ServeWith
	bcastCh                 chan wire.Message
	subCh                   chan subscription
	unsubCh                 chan chan wire.Message
	updateTicker            *time.Ticker
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
	requests                map[uint64]*Request
	pending                 map[netip.Addr][]*Request
	sessions                map[uint64]*Session
	dirty                   map[any]int
	dirtOrder               int
}

// New allocates a JaWS instance with the default configuration.
//
// The returned [Jaws] value is ready for use: static assets are embedded,
// internal goroutines are configured and the request pool is primed. Call
// [Jaws.Close] when the instance is no longer needed to free associated resources.
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
				subCh:                   make(chan subscription, 1),
				unsubCh:                 make(chan chan wire.Message, 1),
				updateTicker:            time.NewTicker(DefaultUpdateInterval),
				kg:                      bufio.NewReader(rand.Reader),
				requests:                make(map[uint64]*Request),
				pending:                 make(map[netip.Addr][]*Request),
				sessions:                make(map[uint64]*Session),
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
	defer jw.mu.RUnlock()
	for _, tl := range jw.tmplookers {
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
func (jw *Jaws) NewRequest(r *http.Request) (rq *Request) {
	remoteIP := jw.clientIP(r)

	jw.mu.Lock()
	defer jw.mu.Unlock()
	jw.limitPendingRequestsLocked(remoteIP)
	for rq == nil {
		jawsKey := jw.nonZeroRandomLocked()
		if _, ok := jw.requests[jawsKey]; !ok {
			rq = jw.getRequestLocked(jawsKey, r)
			jw.requests[jawsKey] = rq
			jw.pending[rq.remoteIP] = append(jw.pending[rq.remoteIP], rq)
		}
	}
	return
}

func (jw *Jaws) limitPendingRequestsLocked(remoteIP netip.Addr) {
	limit := jw.MaxPendingRequestsPerIP
	if limit > 0 {
		for len(jw.pending[remoteIP]) >= limit {
			oldest := jw.pending[remoteIP][0]
			jw.recycleLockedWithCause(oldest, newErrTooManyPendingRequests(remoteIP, limit))
		}
	}
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

func (jw *Jaws) nonZeroRandomLocked() (value uint64) {
	random := make([]byte, 8)
	for value == 0 {
		if _, err := io.ReadFull(jw.kg, random); err != nil {
			panic(err)
		}
		value = binary.LittleEndian.Uint64(random)
	}
	return
}

// UseRequest extracts the JaWS [Request] with the given key from the request
// map if it exists and the HTTP request remote IP matches.
//
// Call it when receiving the WebSocket connection on "/jaws/:key" to get the
// associated [Request], and then call its [Request.ServeHTTP] method to process the
// WebSocket messages.
//
// Returns nil if the key was not found or the IP doesn't match, in which
// case you should return an HTTP "404 Not Found" status.
func (jw *Jaws) UseRequest(jawsKey uint64, r *http.Request) (rq *Request) {
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

// SessionCount returns the number of active sessions.
func (jw *Jaws) SessionCount() (n int) {
	jw.mu.RLock()
	n = len(jw.sessions)
	jw.mu.RUnlock()
	return
}

// Sessions returns a list of all active sessions, which may be nil.
func (jw *Jaws) Sessions() (sessions []*Session) {
	jw.mu.RLock()
	if n := len(jw.sessions); n > 0 {
		sessions = make([]*Session, 0, n)
		for _, sess := range jw.sessions {
			sessions = append(sessions, sess)
		}
	}
	jw.mu.RUnlock()
	return
}

func (jw *Jaws) getSessionLocked(sessionIDs []uint64, remoteIP netip.Addr) *Session {
	for _, sessionID := range sessionIDs {
		if sess, ok := jw.sessions[sessionID]; ok && equalIP(remoteIP, sess.remoteIP) {
			if !sess.isDead() {
				return sess
			}
		}
	}
	return nil
}

func getCookieSessionsIDs(h http.Header, wanted string) (cookies []uint64) {
	for _, line := range h["Cookie"] {
		if strings.Contains(line, wanted) {
			var part string
			line = textproto.TrimString(line)
			for len(line) > 0 {
				part, line, _ = strings.Cut(line, ";")
				if part = textproto.TrimString(part); part != "" {
					name, val, _ := strings.Cut(part, "=")
					name = textproto.TrimString(name)
					if name == wanted {
						if len(val) > 1 && val[0] == '"' && val[len(val)-1] == '"' {
							val = val[1 : len(val)-1]
						}
						if sessionID := assets.JawsKeyValue(val); sessionID != 0 {
							cookies = append(cookies, sessionID)
						}
					}
				}
			}
		}
	}
	return
}

// GetSession returns the [Session] associated with the given [http.Request], or nil.
//
// Sessions are bound to the client IP (see the clientIP method). Behind a reverse
// proxy that connects over loopback, every request appears to come from loopback
// and IP binding is effectively disabled unless [Jaws.TrustForwardedHeaders] is
// enabled so the forwarded client IP is used instead.
func (jw *Jaws) GetSession(r *http.Request) (sess *Session) {
	if r != nil {
		if sessionIDs := getCookieSessionsIDs(r.Header, jw.CookieName); len(sessionIDs) > 0 {
			remoteIP := jw.clientIP(r)
			jw.mu.RLock()
			sess = jw.getSessionLocked(sessionIDs, remoteIP)
			jw.mu.RUnlock()
		}
	}
	return
}

// NewSession creates a new [Session].
//
// Any pre-existing [Session] will be cleared and closed.
// This may call [Session.Close] on an existing session and therefore requires
// the JaWS processing loop ([Jaws.Serve] or [Jaws.ServeWithTimeout]) to be running.
//
// Subsequent [Request] values created with [Jaws.NewRequest] that have the
// cookie set and originate from the same IP will be able to access the [Session].
// The IP comparison is the same loopback-aware, optionally forwarded-header-based
// match used everywhere else; see [Jaws.GetSession] and [Jaws.TrustForwardedHeaders]
// for the reverse-proxy caveat.
func (jw *Jaws) NewSession(w http.ResponseWriter, r *http.Request) (sess *Session) {
	if r != nil {
		if oldSess := jw.GetSession(r); oldSess != nil {
			oldSess.Clear()
			oldSess.Close()
		}
		sess = jw.newSession(w, r)
	}
	return
}

func (jw *Jaws) newSession(w http.ResponseWriter, r *http.Request) (sess *Session) {
	secure := secureheaders.RequestIsSecure(r, jw.TrustForwardedHeaders)
	jw.mu.Lock()
	defer jw.mu.Unlock()
	for sess == nil {
		sessionID := jw.nonZeroRandomLocked()
		if _, ok := jw.sessions[sessionID]; !ok {
			sess = newSession(jw, sessionID, jw.clientIP(r), secure)
			jw.sessions[sessionID] = sess
			if w != nil {
				http.SetCookie(w, &sess.cookie)
			}
			r.AddCookie(&sess.cookie)
		}
	}
	return
}

func (jw *Jaws) deleteSession(sessionID uint64) {
	jw.mu.Lock()
	delete(jw.sessions, sessionID)
	jw.mu.Unlock()
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
					if !strings.HasSuffix(u.Path, jawsurl.Path) {
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

// Broadcast sends a message to all [Request] values.
//
// It must not be called before the JaWS processing loop ([Jaws.Serve] or
// [Jaws.ServeWithTimeout]) is running. Otherwise this call may block once the
// internal broadcast channel fills.
//
// All convenience helpers on [Jaws] that call Broadcast inherit this requirement.
func (jw *Jaws) Broadcast(msg wire.Message) {
	switch msg.Dest.(type) {
	case nil: // send to all requests
	case *Request: // send to that request
	case string: // HTML id (accepted by all requests)
	default:
		expanded, err := tag.TagExpand(nil, msg.Dest)
		jw.MustLog(err)
		for _, tagValue := range expanded {
			// Expanded tags become map keys in the processing loop (wantMessage's
			// lookup in Request.tagMap). A value can pass tag expansion's static
			// comparability check yet be non-comparable at runtime (a comparable
			// struct holding e.g. a func in an interface field), which panics when
			// hashed. NewErrNotComparable runs the runtime value check that
			// ensureUsableTag only performs in debug builds; doing it here rejects a
			// bad Dest before it can panic the Serve goroutine and crash the process.
			if cmperr := tag.NewErrNotComparable(tagValue); cmperr != nil {
				jw.reportMisuse(fmt.Errorf("jaws: Broadcast: %w", cmperr))
				return
			}
		}
		switch len(expanded) {
		case 0:
			// no tags, so no requests will match
			return
		case 1:
			msg.Dest = expanded[0]
		default:
			msg.Dest = expanded
		}
	}
	select {
	case <-jw.Done():
	case jw.bcastCh <- msg:
	}
}

// setDirty marks all Elements that have one or more of the given tags as dirty.
func (jw *Jaws) setDirty(tags []any) {
	jw.mu.Lock()
	// Release the lock with defer so it is freed even if a map insert panics: a tag
	// that passed the static comparability check in ensureUsableTag can still be
	// non-comparable at runtime (a comparable struct holding e.g. a func in an
	// interface field) and panic when used as a map key here.
	defer jw.mu.Unlock()
	for _, tagValue := range tags {
		jw.dirtOrder++
		jw.dirty[tagValue] = jw.dirtOrder
	}
}

// Dirty marks all [Element] values that have one or more of the given tags as dirty.
//
// Note that if any of the tags implement [tag.TagGetter], it will be called
// with a nil [Request]. Prefer using [Request.Dirty] which avoids this.
func (jw *Jaws) Dirty(dirtyTags ...any) {
	// Use TagExpand+MustLog rather than MustTagExpand: with a nil Context the
	// latter panics on an illegal tag even in production, unlike the sibling
	// Request.Dirty and Jaws.Broadcast. Log and continue with the partial result.
	expanded, err := tag.TagExpand(nil, dirtyTags)
	jw.MustLog(err)
	jw.setDirty(expanded)
}

func (jw *Jaws) distributeDirt() int {
	var reqs []*Request
	var dirt []any

	jw.mu.Lock()
	if len(jw.dirty) > 0 {
		dirt = make([]any, 0, len(jw.dirty))
		for k := range jw.dirty {
			dirt = append(dirt, k)
		}
		slices.SortFunc(dirt, func(a, b any) int { return cmp.Compare(jw.dirty[a], jw.dirty[b]) })
		clear(jw.dirty)
		jw.dirtOrder = 0
		reqs = make([]*Request, 0, len(jw.requests))
		for _, rq := range jw.requests {
			reqs = append(reqs, rq)
		}
	}
	jw.mu.Unlock()

	for _, rq := range reqs {
		rq.appendDirtyTags(dirt)
	}
	return len(dirt)
}

// Reload requests all [Request] values to reload their current page.
func (jw *Jaws) Reload() {
	jw.Broadcast(wire.Message{
		What: what.Reload,
	})
}

// isSafeRedirect reports whether rawurl is safe to hand to the browser's
// location.assign, and returns the normalized value to actually send.
//
// Leading and trailing ASCII whitespace and control characters are trimmed, as
// browsers strip them before navigating. Only same-document/relative paths and
// the http and https schemes are permitted; this blocks script-bearing schemes
// such as javascript: and data:, backslashes (which browsers treat as '/'), and
// protocol-relative URLs ("//host/path", "/\host") that would navigate to an
// arbitrary external origin.
func isSafeRedirect(rawurl string) (safe string, ok bool) {
	safe = strings.TrimFunc(rawurl, func(r rune) bool { return r <= ' ' })
	if strings.ContainsRune(safe, '\\') {
		return safe, false
	}
	if u, err := url.Parse(safe); err == nil {
		switch strings.ToLower(u.Scheme) {
		case "":
			if u.Host == "" && !strings.HasPrefix(safe, "//") {
				ok = true
			}
		case "http", "https":
			ok = true
		}
	}
	return
}

// redirectMessage validates url for the browser's location.assign and returns
// the wire.Message to broadcast (Data set to the normalized value); the caller
// sets msg.Dest. If url is unsafe it logs the refusal and returns ok=false. It
// is the single point where the redirect policy and rejection message live, so
// Jaws.Redirect and Request.Redirect cannot drift.
func (jw *Jaws) redirectMessage(url string) (msg wire.Message, ok bool) {
	var safe string
	if safe, ok = isSafeRedirect(url); ok {
		msg = wire.Message{What: what.Redirect, Data: safe}
	} else {
		_ = jw.Log(fmt.Errorf("jaws: refusing unsafe redirect to %q", url))
	}
	return
}

// Redirect requests all [Request] values to navigate to the given URL.
//
// The URL is validated to be a relative path or an http/https URL; script-bearing
// schemes such as javascript: and protocol-relative ("//host") URLs are refused
// and logged rather than sent to the browser.
func (jw *Jaws) Redirect(url string) {
	if msg, ok := jw.redirectMessage(url); ok {
		jw.Broadcast(msg)
	}
}

// Alert sends an alert to all [Request] values.
//
// The lvl argument should be one of Bootstrap's alert levels:
// primary, secondary, success, danger, warning, info, light or dark.
//
// The level and msg are HTML-escaped before being sent, so it is safe to pass
// untrusted text; do not pre-escape it.
func (jw *Jaws) Alert(level, msg string) {
	jw.Broadcast(wire.Message{
		What: what.Alert,
		Data: alertData(level, msg),
	})
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
	const minInterval = time.Millisecond * 10
	const maxInterval = time.Second
	maintenanceInterval := min(requestTimeout/2, maxInterval)
	maintenanceInterval = max(maintenanceInterval, minInterval)

	subs := map[chan wire.Message]*Request{}
	t := time.NewTicker(maintenanceInterval)
	jw.mu.Lock()
	jw.webSocketTimeout = requestTimeout
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
						rq.cancel(fmt.Errorf("%v: broadcast channel full sending %s", rq, msg.String()))
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
	jw.mu.Lock()
	defer jw.mu.Unlock()
	now := time.Now()
	for _, rq := range jw.requests {
		if rq.maintenance(now, requestTimeout) {
			jw.recycleLocked(rq)
		}
	}
	for k, sess := range jw.sessions {
		if sess.isDead() {
			delete(jw.sessions, k)
		}
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

// broadcastTo broadcasts a single wire command to all HTML elements matching
// target. It is the shared body of the public broadcast helpers below, which
// differ only in the What command and how they assemble data.
func (jw *Jaws) broadcastTo(target any, w what.What, data string) {
	jw.Broadcast(wire.Message{
		Dest: target,
		What: w,
		Data: data,
	})
}

// SetInner sends a request to replace the inner HTML of
// all HTML elements matching target.
func (jw *Jaws) SetInner(target any, innerHTML template.HTML) {
	jw.broadcastTo(target, what.Inner, string(innerHTML))
}

// SetAttr sends a request to replace the given attribute value in
// all HTML elements matching target.
//
// The value parameter must be the unescaped logical attribute value. It is sent
// to the browser DOM and used as the value argument to setAttribute().
func (jw *Jaws) SetAttr(target any, attr, value string) {
	jw.broadcastTo(target, what.SAttr, attr+"\n"+value)
}

// RemoveAttr sends a request to remove the given attribute from
// all HTML elements matching target.
func (jw *Jaws) RemoveAttr(target any, attr string) {
	jw.broadcastTo(target, what.RAttr, attr)
}

// SetClass sends a request to set the given class in
// all HTML elements matching target.
func (jw *Jaws) SetClass(target any, cls string) {
	jw.broadcastTo(target, what.SClass, cls)
}

// RemoveClass sends a request to remove the given class from
// all HTML elements matching target.
func (jw *Jaws) RemoveClass(target any, cls string) {
	jw.broadcastTo(target, what.RClass, cls)
}

// SetValue sends a request to set the current input value (in textual form) of
// all HTML elements matching target. It sets the live DOM value/state, not the
// HTML "value" attribute.
func (jw *Jaws) SetValue(target any, value string) {
	jw.broadcastTo(target, what.Value, value)
}

// Insert calls the JavaScript 'insertBefore()' method on
// all HTML elements matching target.
//
// The position parameter 'where' may be either an HTML ID, a child index or the text "null".
// html is trusted HTML, matching [Jaws.SetInner] and [Jaws.Append].
func (jw *Jaws) Insert(target any, where string, html template.HTML) {
	jw.broadcastTo(target, what.Insert, where+"\n"+string(html))
}

// Replace replaces HTML on all HTML elements matching target.
//
// html is trusted HTML, matching [Jaws.SetInner] and [Jaws.Append].
func (jw *Jaws) Replace(target any, html template.HTML) {
	jw.broadcastTo(target, what.Replace, string(html))
}

// Delete removes the HTML element(s) matching target.
func (jw *Jaws) Delete(target any) {
	jw.broadcastTo(target, what.Delete, "")
}

// Append calls the JavaScript appendChild method on all HTML elements matching target.
func (jw *Jaws) Append(target any, html template.HTML) {
	jw.broadcastTo(target, what.Append, string(html))
}

// maybeCompactJSON returns in made safe to embed verbatim in a what.Call wire
// frame, which the client splits on '\n' (frames) and '\t' (order fields). For
// valid JSON, json.Compact strips all insignificant whitespace, including any
// framing-significant tabs and newlines between tokens. When in is not valid JSON
// (for example a caller embedded a raw control byte inside a string literal, which
// is illegal unescaped in JSON), json.Compact fails; rather than passing the raw
// bytes through and corrupting the frame, escape the framing-significant control
// bytes so the payload is always frame-safe (and, as a side effect, valid JSON).
func maybeCompactJSON(in string) string {
	if strings.ContainsAny(in, "\n\t") {
		var b bytes.Buffer
		if err := json.Compact(&b, []byte(in)); err == nil {
			return b.String()
		}
		return jsonControlEscaper.Replace(in)
	}
	return in
}

// jsonControlEscaper turns the raw control bytes that would break WebSocket frame
// and order framing into their JSON escape sequences. These bytes are illegal
// unescaped inside a JSON string literal, so escaping them also yields valid JSON.
var jsonControlEscaper = strings.NewReplacer("\t", `\t`, "\n", `\n`, "\r", `\r`)

var whitespaceRemover = strings.NewReplacer(" ", "", "\n", "", "\t", "")

// JsCall calls the JavaScript function jsfunc with the argument jsonstr
// on all HTML elements matching target.
func (jw *Jaws) JsCall(target any, jsfunc, jsonstr string) {
	jw.broadcastTo(target, what.Call, whitespaceRemover.Replace(jsfunc)+"="+maybeCompactJSON(jsonstr))
}

func (jw *Jaws) getRequestLocked(jawsKey uint64, r *http.Request) (rq *Request) {
	rq = jw.reqPool.Get().(*Request)
	rq.JawsKey = jawsKey
	rq.lastWrite = time.Now()
	rq.initial = r
	rq.ctx, rq.cancelFn = context.WithCancelCause(jw.BaseContext)
	if r != nil {
		rq.remoteIP = jw.clientIP(r)
		if sess := jw.getSessionLocked(getCookieSessionsIDs(r.Header, jw.CookieName), rq.remoteIP); sess != nil {
			sess.addRequest(rq)
			rq.session = sess
		}
	}
	return rq
}

func (jw *Jaws) recycleLockedWithCause(rq *Request, err error) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if rq.JawsKey != 0 {
		if err != nil {
			rq.cancelLocked(err)
		}
		jw.removePendingRequestLocked(rq)
		delete(jw.requests, rq.JawsKey)
		rq.clearLocked()
		jw.reqPool.Put(rq)
	}
}

func (jw *Jaws) recycleLocked(rq *Request) {
	jw.recycleLockedWithCause(rq, nil)
}

func (jw *Jaws) recycle(rq *Request) {
	jw.mu.Lock()
	defer jw.mu.Unlock()
	jw.recycleLocked(rq)
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
					jawsKey := assets.JawsKeyValue(jawsKeyString)
					jw.mu.RLock()
					rq := jw.requests[jawsKey]
					jw.mu.RUnlock()
					if rq != nil {
						if err := rq.writeTailScriptResponse(w); err != nil {
							rq.cancel(err)
						}
						return
					}
				}
			}
		} else if rq := jw.UseRequest(assets.JawsKeyValue(r.URL.Path[6:]), r); rq != nil {
			rq.ServeHTTP(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

type sessioner struct {
	jw *Jaws
	h  http.Handler
}

func (sess sessioner) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if sess.jw.GetSession(r) == nil {
		sess.jw.newSession(w, r)
	}
	sess.h.ServeHTTP(w, r)
}

// Session returns an [http.Handler] that ensures a JaWS [Session] exists before invoking h.
func (jw *Jaws) Session(h http.Handler) http.Handler {
	return sessioner{jw: jw, h: h}
}
