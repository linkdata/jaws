package jaws

import (
	"net/http"
	"net/netip"
	"net/textproto"
	"slices"
	"strings"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/key"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
	"github.com/linkdata/secureheaders"
)

// Session stores server-side per-user state shared by one or more requests.
//
// A Session is bound to the remote IP that created it. Its exported methods are
// safe to call on a nil *Session; those calls return the documented zero value
// or do nothing.
type Session struct {
	jw        *Jaws
	sessionID key.Key
	remoteIP  netip.Addr
	mu        deadlock.RWMutex // protects following
	requests  []*Request
	deadline  time.Time
	cookie    http.Cookie
	data      map[string]any
}

func newSession(jw *Jaws, sessionID key.Key, remoteIP netip.Addr, secure bool) *Session {
	return &Session{
		jw:        jw,
		sessionID: sessionID,
		remoteIP:  remoteIP,
		deadline:  time.Now().Add(time.Minute),
		cookie: http.Cookie{ // #nosec G124 -- Secure is set from the request scheme, and HttpOnly/SameSite are set below.
			Name:     jw.CookieName,
			Path:     "/",
			Value:    sessionID.String(),
			Secure:   secure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
		data: make(map[string]any),
	}
}

func (sess *Session) isDeadLocked() bool {
	return sess.cookie.MaxAge < 0 || (len(sess.requests) == 0 && time.Since(sess.deadline) > 0)
}

func (sess *Session) isDead() (yes bool) {
	sess.mu.RLock()
	yes = sess.isDeadLocked()
	sess.mu.RUnlock()
	return
}

func (sess *Session) addRequest(rq *Request) {
	sess.mu.Lock()
	sess.requests = append(sess.requests, rq)
	sess.mu.Unlock()
}

// delRequest removes rq from the Session. wasClaimed is captured by the caller from
// rq's lifecycle state before it finishes (see Request.killSessionLocked), rather than
// read here, so the grace-window decision does not depend on the order in which the
// finish transition and the session detach happen.
func (sess *Session) delRequest(rq *Request, wasClaimed bool) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	for i := range sess.requests {
		if sess.requests[i] == rq {
			l := len(sess.requests)
			if l > 1 {
				sess.requests[i] = sess.requests[l-1]
			}
			sess.requests[l-1] = nil // release the freed tail slot so it doesn't pin a finished *Request
			sess.requests = sess.requests[:l-1]
			break
		}
	}
	if wasClaimed {
		// A claimed request's WebSocket has ended; grant a fresh grace window so
		// other tabs or a reconnect can re-attach before the session expires. This
		// must fire even when other requests remain attached, otherwise an aged
		// session whose last departing request is an unclaimed bootstrap render
		// would be reaped with its stale deadline despite recent live activity.
		sess.deadline = time.Now().Add(time.Minute)
	}
	// For an unclaimed request (its bootstrap render finished before the
	// WebSocket connected) leave the existing deadline intact: the creation-time
	// grace window (see newSession), or the window left by a claimed request that
	// departed earlier, governs the session's lifetime until a WebSocket attaches.
}

// Jaws returns the [Jaws] instance of the [Session], or nil.
// It is safe to call on a nil [Session].
func (sess *Session) Jaws() (jw *Jaws) {
	if sess != nil {
		jw = sess.jw
	}
	return
}

// Get returns the value associated with the key, or nil.
// It is safe to call on a nil [Session].
func (sess *Session) Get(key string) (value any) {
	if sess != nil {
		sess.mu.RLock()
		value = sess.data[key]
		sess.mu.RUnlock()
	}
	return
}

// Set sets a value to be associated with the key.
// If value is nil, the key is removed from the session.
// It is safe to call on a nil [Session].
func (sess *Session) Set(key string, value any) {
	if sess != nil {
		sess.mu.Lock()
		if value == nil {
			delete(sess.data, key)
		} else {
			sess.data[key] = value
		}
		sess.mu.Unlock()
	}
}

// ID returns the session ID, a 64-bit random value.
// It is safe to call on a nil [Session], in which case it returns zero.
func (sess *Session) ID() (id uint64) {
	if sess != nil {
		id = uint64(sess.sessionID)
	}
	return
}

// CookieValue returns the session cookie value.
// It is safe to call on a nil [Session], in which case it returns an empty string.
func (sess *Session) CookieValue() (s string) {
	if sess != nil {
		s = sess.cookie.Value
	}
	return
}

// IP returns the remote IP the session is bound to, or the zero [netip.Addr] if unset.
// It is safe to call on a nil [Session], in which case it returns the zero [netip.Addr].
func (sess *Session) IP() (ip netip.Addr) {
	if sess != nil {
		ip = sess.remoteIP
	}
	return
}

// Cookie returns a cookie for the [Session]. Returns a delete cookie if the [Session] is expired.
// It is safe to call on a nil [Session], in which case it returns nil.
func (sess *Session) Cookie() (cookie *http.Cookie) {
	if sess != nil {
		cookie = &http.Cookie{} // #nosec G124 -- copied from sess.cookie before returning.
		sess.mu.RLock()
		*cookie = sess.cookie
		if sess.isDeadLocked() {
			cookie.MaxAge = -1
		}
		sess.mu.RUnlock()
	}
	return
}

// addCookie adds sess's cookie to w and r while sess is current and live.
func (sess *Session) addCookie(w http.ResponseWriter, r *http.Request) {
	var h http.Header
	if w != nil {
		// ResponseWriter.Header is caller code and may re-enter Jaws, including
		// by closing sess, so call it before taking either core lock below.
		h = w.Header()
	}
	jw := sess.jw
	jw.mu.RLock()
	defer jw.mu.RUnlock()
	sess.mu.RLock()
	defer sess.mu.RUnlock()
	if jw.sessions[sess.sessionID] == sess && !sess.isDeadLocked() {
		cookie := sess.cookie
		if h != nil {
			if v := cookie.String(); v != "" {
				// Header.Add and Request.AddCookie mutate concrete header maps
				// without invoking caller code. Keep both under these read locks so
				// cookie publication precedes a losing Session.Close.
				h.Add("Set-Cookie", v)
			}
		}
		r.AddCookie(&cookie)
	}
}

// Close invalidates and expires the [Session].
// Future [Request] values won't be able to associate with it, and [Session.Cookie] will return a deletion cookie.
//
// Existing [Request] values already associated with the [Session] will ask the
// browser to reload the pages. This holds even for a [Request] whose WebSocket
// has not connected yet: the reload is queued on the [Request] and delivered when
// it connects.
// Key/value pairs in the [Session] are left unmodified; use [Session.Clear] to remove all of them.
//
// It must not be called before the JaWS processing loop ([Jaws.Serve] or
// [Jaws.ServeWithTimeout]) is running, because the wake-up broadcasts may block.
//
// Returns a cookie to be sent to the client browser that will delete the browser cookie.
// It is safe to call on a nil [Session], in which case it returns nil; for any
// non-nil [Session] it returns a non-nil deletion cookie.
func (sess *Session) Close() (cookie *http.Cookie) {
	if sess != nil {
		sess.jw.deleteSession(sess.sessionID)

		sess.mu.Lock()
		sess.cookie.MaxAge = -1 // #nosec G124 -- marks the already initialized session cookie for deletion.
		requests := sess.requests
		sess.requests = nil
		cookie = new(http.Cookie)
		*cookie = sess.cookie
		sess.mu.Unlock()

		// deadSession queues the reload directly onto each Request, covering those
		// whose WebSocket has not subscribed yet. This key-targeted Update is only a
		// wake-up: it makes an already-running process loop iterate and flush the
		// queued reload. handleBroadcast resolves a key destination to no elements,
		// so the Update itself performs no browser operation.
		msg := wire.Message{What: what.Update}
		for _, rq := range requests {
			if k := rq.deadSession(sess); k != 0 {
				msg.Dest = k
				sess.jw.Broadcast(msg)
			}
		}
	}
	return
}

// Reload calls [Session.Broadcast] with a message asking browsers to reload the page.
// See [Session.Broadcast] for the processing-loop requirement.
// It is safe to call on a nil [Session].
func (sess *Session) Reload() {
	sess.Broadcast(wire.Message{What: what.Reload})
}

// Clear removes all key/value pairs from the session.
// It is safe to call on a nil [Session].
func (sess *Session) Clear() {
	if sess != nil {
		sess.mu.Lock()
		clear(sess.data)
		sess.mu.Unlock()
	}
}

// Requests returns a list of the [Request] values using this [Session].
//
// The returned slice is a snapshot. Its Request pointers are not pinned and may
// become stale immediately; see [Request].
// It is safe to call on a nil [Session].
func (sess *Session) Requests() (requests []*Request) {
	if sess != nil {
		sess.mu.RLock()
		requests = slices.Clone(sess.requests)
		sess.mu.RUnlock()
	}
	return
}

// Broadcast attempts to send a message to all active [Request] values using this session.
//
// It must not be called before the JaWS processing loop ([Jaws.Serve] or
// [Jaws.ServeWithTimeout]) is running. Otherwise this call may block.
// It is safe to call on a nil [Session].
func (sess *Session) Broadcast(msg wire.Message) {
	if sess != nil {
		// Snapshot the requests under the lock (via Requests), then broadcast
		// outside it: jw.Broadcast can block on the broadcast channel under
		// backpressure, and holding sess.mu across that send would stall every
		// other session reader and writer. This mirrors Session.Close.
		for _, rq := range sess.Requests() {
			if k := rq.sessionDestKey(sess); k != 0 {
				msg.Dest = k
				sess.jw.Broadcast(msg)
			}
		}
	}
}

// SessionCount returns the number of active sessions.
func (jw *Jaws) SessionCount() (n int) {
	jw.mu.RLock()
	n = len(jw.sessions)
	jw.mu.RUnlock()
	return
}

// Sessions returns a snapshot of all registered sessions, which may be nil.
//
// Auto-created [Session] values are registered only after their initiating
// [Request] is associated.
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

func (jw *Jaws) getSessionByIDLocked(sessionID key.Key, remoteIP netip.Addr) (sess *Session) {
	if found, ok := jw.sessions[sessionID]; ok && equalIP(remoteIP, found.remoteIP) && !found.isDead() {
		sess = found
	}
	return
}

func (jw *Jaws) getSessionLocked(sessionIDs []key.Key, remoteIP netip.Addr) (sess *Session) {
	for _, sessionID := range sessionIDs {
		if sess = jw.getSessionByIDLocked(sessionID, remoteIP); sess != nil {
			break
		}
	}
	return
}

func getCookieSessionsIDs(h http.Header, wanted string) (cookies []key.Key) {
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
						if sessionID, tail := key.Parse(val); sessionID != 0 && tail == "" {
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
// All live pre-existing [Session] values referenced by matching cookies and
// bound to the request's client IP are cleared and closed. Each is closed with
// [Session.Close], so the JaWS processing loop ([Jaws.Serve] or
// [Jaws.ServeWithTimeout]) must be running.
//
// Subsequent [Request] values created with [Jaws.NewRequest] that have the
// cookie set and originate from the same IP will be able to access the [Session].
// The IP comparison is the same loopback-aware, optionally forwarded-header-based
// match used everywhere else; see [Jaws.GetSession] and [Jaws.TrustForwardedHeaders]
// for the reverse-proxy caveat.
//
// If the new [Session] remains current and live during cookie publication, its
// cookie is written to w when w is non-nil and added to r itself. This makes the
// [Session] visible to [Jaws.GetSession] and [Jaws.NewRequest] for the remainder
// of the same HTTP request. If a concurrent [Session.Close] wins first, neither
// w nor r receives its live cookie.
//
// It returns nil and has no effect if r is nil; w may be nil.
//
// It panics if the system CSPRNG ([crypto/rand]) fails while generating the session
// ID, which does not happen on supported platforms.
func (jw *Jaws) NewSession(w http.ResponseWriter, r *http.Request) (sess *Session) {
	if r != nil {
		if sessionIDs := getCookieSessionsIDs(r.Header, jw.CookieName); len(sessionIDs) > 0 {
			remoteIP := jw.clientIP(r)
			for _, sessionID := range sessionIDs {
				jw.mu.RLock()
				oldSess := jw.getSessionByIDLocked(sessionID, remoteIP)
				jw.mu.RUnlock()
				if oldSess != nil {
					oldSess.Clear()
					oldSess.Close()
				}
			}
		}
		sess = jw.newSession(w, r)
	}
	return
}

func (jw *Jaws) newSession(w http.ResponseWriter, r *http.Request) (sess *Session) {
	secure := secureheaders.RequestIsSecure(r, jw.TrustForwardedHeaders)
	func() {
		jw.mu.Lock()
		defer jw.mu.Unlock()
		sess = jw.newSessionLocked(jw.clientIP(r), secure)
		jw.sessions[sess.sessionID] = sess
	}()
	sess.addCookie(w, r)
	return
}

// newSessionLocked allocates a Session whose ID is absent from jw.sessions.
//
// The caller must hold jw.mu and publish the Session before releasing it.
func (jw *Jaws) newSessionLocked(remoteIP netip.Addr, secure bool) (sess *Session) {
	for sess == nil {
		sessionID := jw.nonZeroRandomLocked()
		if _, ok := jw.sessions[sessionID]; !ok {
			sess = newSession(jw, sessionID, remoteIP, secure)
		}
	}
	return
}

func (jw *Jaws) deleteSession(sessionID key.Key) {
	jw.mu.Lock()
	delete(jw.sessions, sessionID)
	jw.mu.Unlock()
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

// SessionMiddleware returns a session-creating [http.Handler].
//
// Before invoking h, it creates a JaWS [Session] when the request has none. If
// a concurrent [Session.Close] wins the new Session's cookie publication, h
// runs without that Session or its live cookie.
//
// It is distinct from the session accessors:
// [Jaws.GetSession] and [Request.Session] look up an existing [Session], while
// this wraps a handler. It composes with [Jaws.SecureHeadersMiddleware].
func (jw *Jaws) SessionMiddleware(h http.Handler) http.Handler {
	return sessioner{jw: jw, h: h}
}
