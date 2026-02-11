package core

import "sync"

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
