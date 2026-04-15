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

func newTestSetter[T comparable](val T) *testSetter[T] {
	return &testSetter[T]{
		val:       val,
		setCalled: make(chan struct{}),
		getCalled: make(chan struct{}),
	}
}

func (ts *testSetter[T]) Get() (val T) {
	ts.mu.Lock()
	val = ts.val
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) Set(val T) {
	ts.mu.Lock()
	ts.val = val
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

func (ts *testSetter[T]) JawsGet(*jaws.Element) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[T]) JawsSet(_ *jaws.Element, val T) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = jaws.ErrValueUnchanged
		}
		ts.val = val
	}
	return
}

func (ts *testSetter[string]) JawsGetString(*jaws.Element) (val string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[any]) JawsGetAny(*jaws.Element) (val any) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[any]) JawsSetAny(_ *jaws.Element, val any) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = jaws.ErrValueUnchanged
		}
		ts.val = val
	}
	return
}

func (ts *testSetter[T]) JawsGetHTML(*jaws.Element) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

type selfTagger struct{}

func (st *selfTagger) JawsGetTag(tag.Context) any {
	return st
}
