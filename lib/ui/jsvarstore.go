package ui

import (
	"cmp"
	"slices"
	"sync"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/wire"
)

// JsVarStore groups path updates for bindings over one Go value.
//
// Create one store for each synchronized backing pointer, then call [JsVarStore.NewJsVar]
// for each request-scoped binding. The store coalesces writes to the same path
// across those bindings and broadcasts the latest value at most once per
// [jaws.DefaultUpdateInterval]. Different paths retain the order of their latest
// writes. A store may be used with multiple [jaws.Jaws] instances; a write is
// broadcast through its element's instance, and updates from separate instances
// are queued separately. Changes made directly to the Go value are not tracked;
// only writes through a binding are queued.
//
// Create a store with [NewJsVarStore]; its zero value is not ready for use.
// The bound value and its locker must remain valid for the store's lifetime.
// JsVarStore is safe for concurrent use and must not be copied after first use.
type JsVarStore[T any] struct {
	locker  bind.RWLocker
	ptr     *T
	setMu   sync.Mutex
	pending jsVarPendingSet
}

// NewJsVarStore creates a shared store over v protected by l.
//
// The pointer v may be nil; bindings then have the same nil-pointer behavior as
// [NewJsVar].
func NewJsVarStore[T any](l sync.Locker, v *T) *JsVarStore[T] {
	return &JsVarStore[T]{locker: bind.AsRWLocker(l), ptr: v}
}

// NewJsVar creates a fresh request-scoped binding over the store's value.
//
// Configure the returned [JsVar.ClientCheck] before using the binding if browser
// writes need validation. Do not reassign its Ptr or RWLocker.
func (store *JsVarStore[T]) NewJsVar() *JsVar[T] {
	return &JsVar[T]{RWLocker: store.locker, Ptr: store.ptr, setMu: &store.setMu, pending: &store.pending}
}

type jsVarPendingKey struct {
	jaws *jaws.Jaws
	path string
}

type jsVarPending struct {
	jaws  *jaws.Jaws
	msg   wire.Message
	order int
}

type jsVarPendingSet struct {
	updates   map[jsVarPendingKey]jsVarPending
	seq       int
	scheduled bool
}

// queue requires lock to be held. The same lock protects the delayed flush.
func (pending *jsVarPendingSet) queue(lock *sync.Mutex, jw *jaws.Jaws, path string, msg wire.Message) {
	if pending.updates == nil {
		pending.updates = make(map[jsVarPendingKey]jsVarPending)
	}
	pending.seq++
	pending.updates[jsVarPendingKey{jaws: jw, path: path}] = jsVarPending{jaws: jw, msg: msg, order: pending.seq}
	if !pending.scheduled {
		pending.scheduled = true
		time.AfterFunc(jaws.DefaultUpdateInterval, func() {
			lock.Lock()
			defer lock.Unlock()
			pending.flush()
		})
	}
}

// flush requires the queue's lock to be held so later writes cannot overtake it.
func (pending *jsVarPendingSet) flush() {
	updates := make([]jsVarPending, 0, len(pending.updates))
	for _, update := range pending.updates {
		updates = append(updates, update)
	}
	slices.SortFunc(updates, func(a, b jsVarPending) int { return cmp.Compare(a.order, b.order) })
	pending.updates = nil
	pending.seq = 0
	pending.scheduled = false
	for _, update := range updates {
		update.jaws.Broadcast(update.msg)
	}
}
