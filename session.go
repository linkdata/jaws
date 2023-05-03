package jaws

import (
	"net"
	"net/http"
	"time"

	"github.com/linkdata/deadlock"
)

const sessionRefreshSeconds = 60
const sessionHeadSuffix = `";var jawsSession=60;</script>`

type Session struct {
	jw        *Jaws
	sessionID uint64
	remoteIP  net.IP
	mu        deadlock.RWMutex // protects following
	expires   time.Time
	data      map[string]interface{}
}

func newSession(jw *Jaws, sessionID uint64, remoteIP net.IP, expires time.Time) *Session {
	return &Session{
		jw:        jw,
		sessionID: sessionID,
		remoteIP:  remoteIP,
		expires:   expires,
		data:      make(map[string]interface{}),
	}
}

// Get returns the value associated with the key, or nil.
// It is safe to call on a nil Session.
func (sess *Session) Get(key string) (val interface{}) {
	if sess != nil {
		sess.mu.RLock()
		val = sess.data[key]
		sess.mu.RUnlock()
	}
	return
}

// Set sets a value to be associated with the key.
// If value is nil, the key is removed from the session.
// It is safe to call on a nil Session.
func (sess *Session) Set(key string, val interface{}) {
	if sess != nil {
		sess.mu.Lock()
		if val == nil {
			delete(sess.data, key)
		} else {
			sess.data[key] = val
		}
		sess.mu.Unlock()
	}
}

// GetExpires gets the Session expiry time.
// It is safe to call on a nil or closed Session, in which case it returns a zero time.
func (sess *Session) GetExpires() (when time.Time) {
	if sess != nil {
		sess.mu.RLock()
		when = sess.expires
		sess.mu.RUnlock()
	}
	return
}

// SetExpires sets the Session expiry time.
// Attempts to set a zero time will be ignored; use Close() to close the session.
// It is safe to call on a nil Session.
func (sess *Session) SetExpires(when time.Time) {
	if sess != nil && !when.IsZero() {
		sess.mu.Lock()
		sess.expires = when
		sess.mu.Unlock()
	}
}

// ID returns the session ID, a 64-bit random value.
// It is safe to call on a nil Session, in which case it returns zero.
func (sess *Session) ID() (id uint64) {
	if sess != nil {
		id = sess.sessionID
	}
	return
}

// IP returns the remote IP the session is bound to (which may be nil).
// It is safe to call on a nil Session, in which case it returns nil.
func (sess *Session) IP() (ip net.IP) {
	if sess != nil {
		ip = sess.remoteIP
	}
	return
}

// CookieValue returns the cookie value for the Session.
// It is safe to call on a nil Session, in which case it returns an empty string.
func (sess *Session) CookieValue() (val string) {
	if sess != nil {
		val = JawsKeyString(sess.sessionID)
	}
	return
}

func (sess *Session) jid() string {
	return "  " + JawsKeyString(sess.sessionID)
}

func (sess *Session) cookieLocked() *http.Cookie {
	var maxAge int
	expires := sess.expires
	if expires.IsZero() || isExpired(expires, 0) {
		maxAge = -1
		expires = time.Time{}
	}
	return &http.Cookie{
		Name:     sess.jw.CookieName,
		Path:     "/",
		Value:    JawsKeyString(sess.sessionID),
		MaxAge:   maxAge,
		Expires:  expires,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// Cookie returns the cookie for the Session. Returns a delete cookie if the Session is expired.
// It is safe to call on a nil Session, in which case it returns nil.
func (sess *Session) Cookie() (cookie *http.Cookie) {
	if sess != nil {
		sess.mu.RLock()
		cookie = sess.cookieLocked()
		sess.mu.RUnlock()
	}
	return
}

// Refresh ensures the cookie expiry for the session isn't too short.
//
// Returns a session cookie to be set if it's expiry time was updated, or nil.
// It is safe to call on a nil Session, in which case it returns nil.
func (sess *Session) Refresh() (cookie *http.Cookie) {
	if sess != nil {
		expires := sess.GetExpires()
		if !expires.IsZero() { // don't refresh sessions being deleted
			if isExpired(expires, time.Second*sessionRefreshSeconds*2) {
				sess.mu.Lock()
				sess.expires = expires.Add(time.Second * sessionRefreshSeconds * 3)
				cookie = sess.cookieLocked()
				sess.mu.Unlock()
			}
		}
	}
	return
}

// Close invalidates and expires the Session.
// Future Requests won't be able to associate with it, and Cookie() will return a deletion cookie.
//
// Existing Requests already associated with the Session will ask the browser to reload the pages.
// Key/value pairs in the Session are left unmodified, you can use `Session.Clear()` to remove all of them.
//
// Returns the a cookie to be sent to the client browser that will delete the browser cookie.
// Returns nil if the session was not found or is already closed.
// It is safe to call on a nil Session.
func (sess *Session) Close() (cookie *http.Cookie) {
	if sess != nil {
		var deleteID uint64
		sess.mu.Lock()
		if !sess.expires.IsZero() {
			deleteID = sess.sessionID
			sess.expires = time.Time{}
			cookie = sess.cookieLocked()
		}
		sess.mu.Unlock()
		if deleteID != 0 {
			sess.jw.deleteSession(deleteID)
			sess.Reload()
		}
	}
	return
}

// Clear removes all key/value pairs from the session.
// It is safe to call on a nil Session.
func (sess *Session) Clear() {
	if sess != nil {
		sess.mu.Lock()
		for k := range sess.data {
			delete(sess.data, k)
		}
		sess.mu.Unlock()
	}
}

// Reload sends a message to all Requests using this session to reload their webpage.
// It is safe to call on a nil Session.
func (sess *Session) Reload() {
	if sess != nil {
		sess.jw.Broadcast(&Message{
			Elem: sess.jid(),
			What: "reload",
		})
	}
}

func isExpired(t time.Time, d time.Duration) bool {
	return time.Since(t.Add(-d)) >= 0
}
