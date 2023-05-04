package jaws

import (
	"net"
	"net/http"
	"time"

	"github.com/linkdata/deadlock"
)

type Session struct {
	jw        *Jaws
	sessionID uint64
	remoteIP  net.IP
	mu        deadlock.RWMutex // protects following
	requests  []*Request
	deadline  time.Time
	cookie    http.Cookie
	data      map[string]interface{}
}

func newSession(jw *Jaws, sessionID uint64, remoteIP net.IP) *Session {
	return &Session{
		jw:        jw,
		sessionID: sessionID,
		remoteIP:  remoteIP,
		deadline:  time.Now().Add(time.Minute),
		cookie: http.Cookie{
			Name:     jw.CookieName,
			Path:     "/",
			Value:    JawsKeyString(sessionID),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
		data: make(map[string]interface{}),
	}
}

func (sess *Session) isDead() (yes bool) {
	sess.mu.RLock()
	yes = len(sess.requests) == 0 && time.Since(sess.deadline) < 0
	sess.mu.RUnlock()
	return
}

func (sess *Session) addRequest(rq *Request) {
	sess.mu.Lock()
	sess.requests = append(sess.requests, rq)
	sess.mu.Unlock()
}

func (sess *Session) delRequest(rq *Request) {
	sess.mu.Lock()
	for i := range sess.requests {
		if sess.requests[i] == rq {
			l := len(sess.requests)
			if l > 1 {
				sess.requests[i] = sess.requests[l-1]
			}
			sess.requests = sess.requests[:l-1]
			break
		}
	}
	if len(sess.requests) == 0 {
		sess.deadline = time.Now().Add(time.Minute)
	}
	sess.mu.Unlock()
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
	return &http.Cookie{
		Name:     sess.jw.CookieName,
		Path:     "/",
		Value:    JawsKeyString(sess.sessionID),
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
		cookie = &http.Cookie{}
		sess.mu.Lock()
		*cookie = sess.cookie
		cookie.MaxAge = -1
		deleteID = sess.sessionID
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
