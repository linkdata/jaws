package jaws

import (
	"net"
	"net/http"
	"time"

	"github.com/linkdata/deadlock"
)

type Session struct {
	name      string
	sessionID uint64
	remoteIP  net.IP
	mu        deadlock.RWMutex // protects following
	expires   time.Time
	data      map[string]interface{}
}

func newSession(name string, sessionID uint64, remoteIP net.IP, expires time.Time) *Session {
	return &Session{
		name:      name,
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

// GetExpires gets the session expiry time.
// It is safe to call on a nil Session, in which case it returns a zero time.
func (sess *Session) GetExpires() (when time.Time) {
	if sess != nil {
		sess.mu.RLock()
		when = sess.expires
		sess.mu.RUnlock()
	}
	return
}

// SetExpires sets a sessions expiry time.
// It is safe to call on a nil Session.
func (sess *Session) SetExpires(when time.Time) {
	if sess != nil {
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

// Cookie returns the cookie for the Session. Returns a delete cookie if the Session is expired.
// It is safe to call on a nil Session, in which case it returns nil.
func (sess *Session) Cookie() (cookie *http.Cookie) {
	if sess != nil {
		expires := sess.GetExpires()
		cookie = &http.Cookie{
			Name:     sess.name,
			Path:     "/",
			Value:    sess.CookieValue(),
			Expires:  expires,
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		}
		if isExpired(expires, 0) {
			cookie.MaxAge = -1
			cookie.Expires = time.Time{}
		}
	}
	return
}

// Refresh ensures the cookie expiry is at least `minAge` seconds in the future.
// Returns a session cookie to be set if it's expiry time was updated, or nil.
// It is safe to call on a nil Session, in which case it returns nil.
func (sess *Session) Refresh(minAge, maxAge int) (cookie *http.Cookie) {
	if sess != nil {
		expires := sess.GetExpires()
		if !expires.IsZero() && isExpired(expires, minAge) {
			sess.SetExpires(time.Now().Add(time.Second * time.Duration(maxAge)))
			cookie = sess.Cookie()
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

func isExpired(t time.Time, minAge int) bool {
	return t.IsZero() || time.Since(t.Add(time.Second*time.Duration(-minAge))) >= 0
}
