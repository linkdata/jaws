package bind

import (
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

type testSetter[T comparable] struct {
	mu        deadlock.Mutex
	val       T
	err       error
	setCount  int
	getCount  int
	setCalled chan struct{}
	getCalled chan struct{}
}

func newTestSetter[T comparable](value T) *testSetter[T] {
	return &testSetter[T]{
		val:       value,
		setCalled: make(chan struct{}),
		getCalled: make(chan struct{}),
	}
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

func (ts *testSetter[T]) Err() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.err
}

func (ts *testSetter[T]) SetErr(err error) {
	ts.mu.Lock()
	ts.err = err
	ts.mu.Unlock()
}

func (ts *testSetter[T]) SetCount() (n int) {
	ts.mu.Lock()
	n = ts.setCount
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) GetCount() (n int) {
	ts.mu.Lock()
	n = ts.getCount
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) JawsGet(elem *jaws.Element) (value T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

func (ts *testSetter[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == value {
			err = jaws.ErrValueUnchanged
		}
		ts.val = value
	}
	return
}

func (ts *testSetter[string]) JawsGetString(elem *jaws.Element) (value string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

func (ts *testSetter[any]) JawsGetAny(elem *jaws.Element) (value any) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

func (ts *testSetter[any]) JawsSetAny(elem *jaws.Element, value any) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == value {
			err = jaws.ErrValueUnchanged
		}
		ts.val = value
	}
	return
}

func (ts *testSetter[T]) JawsGetHTML(elem *jaws.Element) (value T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

type selfTagger struct{}

func (st *selfTagger) JawsGetTag(tag.Context) any {
	return st
}
