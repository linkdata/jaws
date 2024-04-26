package jaws

type RLocker interface {
	RLock()
	RUnlock()
}
