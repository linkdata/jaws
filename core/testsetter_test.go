package core

import (
	"github.com/linkdata/deadlock"
)

type testSetter[T comparable] struct {
	mu        deadlock.Mutex
	val       T
	err       error
	setCount  int
	setCalled chan struct{}
	getCount  int
	getCalled chan struct{}
}

func newTestSetter[T comparable](val T) *testSetter[T] {
	return &testSetter[T]{
		val:       val,
		setCalled: make(chan struct{}),
		getCalled: make(chan struct{}),
	}
}

func (ts *testSetter[T]) Get() (s T) {
	ts.mu.Lock()
	s = ts.val
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) Set(s T) {
	ts.mu.Lock()
	ts.val = s
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

func (ts *testSetter[T]) JawsGet(e *Element) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[T]) JawsSet(e *Element, val T) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = ErrValueUnchanged
		}
		ts.val = val
	}
	return
}

func (ts *testSetter[string]) JawsGetString(e *Element) (val string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[any]) JawsGetAny(e *Element) (val any) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[any]) JawsSetAny(e *Element, val any) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = ErrValueUnchanged
		}
		ts.val = val
	}
	return
}

func (ts *testSetter[T]) JawsGetHTML(e *Element) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}
