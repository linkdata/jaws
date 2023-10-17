package jaws

import (
	"net"
	"net/http"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
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

func (sess *Session) delRequest(rq *Request) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
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

// CookieValue returns the session cookie value.
// It is safe to call on a nil Session, in which case it returns an empty string.
func (sess *Session) CookieValue() (s string) {
	if sess != nil {
		s = sess.cookie.Value
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

// Cookie returns a cookie for the Session. Returns a delete cookie if the Session is expired.
// It is safe to call on a nil Session, in which case it returns nil.
func (sess *Session) Cookie() (cookie *http.Cookie) {
	if sess != nil {
		cookie = &http.Cookie{}
		sess.mu.RLock()
		*cookie = sess.cookie
		if sess.isDeadLocked() {
			cookie.MaxAge = -1
		}
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
// Returns a cookie to be sent to the client browser that will delete the browser cookie.
// Returns nil if the session was not found or is already closed.
// It is safe to call on a nil Session.
func (sess *Session) Close() (cookie *http.Cookie) {
	if sess != nil {
		sess.jw.deleteSession(sess.sessionID)
		sess.mu.Lock()
		sess.cookie.MaxAge = -1
		sess.broadcastLocked(Message{What: what.Reload})
		sess.requests = sess.requests[:0]
		cookie = &sess.cookie
		sess.mu.Unlock()
	}
	return
}

// Reload calls Broadcast with a message asking browsers to reload the page.
func (sess *Session) Reload() {
	sess.Broadcast(Message{What: what.Reload})
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

func (sess *Session) broadcastLocked(msg Message) {
	var retry []*Request
	for _, rq := range sess.requests {
		select {
		case rq.sendCh <- msg:
		default:
			retry = append(retry, rq)
		}
	}
	for _, rq := range retry {
		msg.Dest = rq
		sess.jw.Broadcast(msg)
	}
}

// Broadcast attempts to send a message to all Requests using this session.
// It is safe to call on a nil Session.
func (sess *Session) Broadcast(msg Message) {
	if sess != nil {
		sess.mu.RLock()
		defer sess.mu.RUnlock()
		sess.broadcastLocked(msg)
	}
}
