package bind

import "sync"

// RWLocker is the subset of [sync.RWMutex] used by binders.
type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
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
