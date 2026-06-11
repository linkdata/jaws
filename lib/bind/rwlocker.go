package bind

import "sync"

// RWLocker is the subset of [sync.RWMutex] used by binders.
type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

// AsRWLocker returns an [RWLocker] backed by l.
//
// If l already implements [RWLocker] it is returned unchanged; otherwise its
// Lock and Unlock are used for both read and write locking. A nil l yields an
// RWLocker that panics when locked.
func AsRWLocker(l sync.Locker) RWLocker {
	if rl, ok := l.(RWLocker); ok {
		return rl
	}
	return rwlocker{l}
}

type rwlocker struct {
	sync.Locker
}

func (l rwlocker) RLock() {
	l.Lock()
}

func (l rwlocker) RUnlock() {
	l.Unlock()
}
