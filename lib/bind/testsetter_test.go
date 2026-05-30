package bind

import (
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// testSetter is a minimal getter/setter fixture for the bind tests. It only
// tracks a value guarded by a mutex; the previous error-injection, call counters
// and "called" signal channels were removed because no test asserted them.
// Error propagation is exercised through a SetHook in TestBind_Hook_Set instead.
type testSetter[T comparable] struct {
	mu  deadlock.Mutex
	val T
}

func newTestSetter[T comparable](value T) *testSetter[T] {
	return &testSetter[T]{val: value}
}

func (ts *testSetter[T]) Get() (value T) {
	ts.mu.Lock()
	value = ts.val
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) Set(value T) {
	ts.mu.Lock()
	ts.val = value
	ts.mu.Unlock()
}

func (ts *testSetter[T]) JawsGet(elem *jaws.Element) (value T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.val
}

func (ts *testSetter[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.val == value {
		return jaws.ErrValueUnchanged
	}
	ts.val = value
	return nil
}

func (ts *testSetter[string]) JawsGetString(elem *jaws.Element) (value string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.val
}

func (ts *testSetter[any]) JawsGetAny(elem *jaws.Element) (value any) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.val
}

func (ts *testSetter[any]) JawsSetAny(elem *jaws.Element, value any) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.val == value {
		return jaws.ErrValueUnchanged
	}
	ts.val = value
	return nil
}

func (ts *testSetter[T]) JawsGetHTML(elem *jaws.Element) (value T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.val
}

type selfTagger struct{}

func (st *selfTagger) JawsGetTag(tag.Context) any {
	return st
}
