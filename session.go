package jaws

import (
	"net"

	"github.com/linkdata/deadlock"
)

type session struct {
	sessionID uint64
	remoteIP  net.IP
	mu        deadlock.RWMutex // protects following
	data      map[string]interface{}
}

func newSession(sessionID uint64, remoteIP net.IP) *session {
	return &session{
		sessionID: sessionID,
		remoteIP:  remoteIP,
		data:      make(map[string]interface{}),
	}
}

func (sess *session) isRemoteOk(remoteIP net.IP) bool {
	return sess != nil && sess.remoteIP.Equal(remoteIP)
}

// get returns the value associated with the key, or nil.
func (sess *session) get(key string) (val interface{}) {
	if sess != nil {
		sess.mu.RLock()
		val = sess.data[key]
		sess.mu.RUnlock()
	}
	return
}

// set sets a value to be associated with the key.
// If value is nil, the key is removed from the session.
func (sess *session) set(key string, val interface{}) {
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
