package ui

import "sync"

type rwlocker struct {
	sync.Locker
}

func (l rwlocker) RLock() {
	l.Lock()
}

func (l rwlocker) RUnlock() {
	l.Unlock()
}
