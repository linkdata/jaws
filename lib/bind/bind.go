package bind

import (
	"sync"
)

// New returns a [Binder] with l protecting the value pointed to by p.
//
// T must be strictly comparable. Interface types, including any, are
// unsupported; an array's element type and every struct field type must also be
// strictly comparable, regardless of the bound values. The default
// [Binder.JawsSetLocked] comparison may panic when T satisfies the comparable
// constraint but is not strictly comparable.
//
// If l implements [RWLocker], reads use its read lock. Otherwise reads and
// writes both use l. The pointer p is also exposed as the UI tag.
//
// Both l and p must be non-nil; a Binder created with a nil locker or nil
// pointer panics on first use.
func New[T comparable](l sync.Locker, p *T) Binder[T] {
	return &binder[T]{RWLocker: AsRWLocker(l), ptr: p}
}
