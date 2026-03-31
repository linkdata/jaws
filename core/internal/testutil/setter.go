package testutil

import "github.com/linkdata/deadlock"

// Setter is a generic test helper implementing Jaws getter/setter shapes.
type Setter[T comparable, E any] struct {
	mu                deadlock.Mutex
	val               T
	err               error
	errValueUnchanged error
	setCount          int
	setCalled         chan struct{}
	getCount          int
	getCalled         chan struct{}
}

// NewSetter creates a Setter initialized with val.
func NewSetter[T comparable, E any](val T, errValueUnchanged error) *Setter[T, E] {
	return &Setter[T, E]{
		val:               val,
		errValueUnchanged: errValueUnchanged,
		setCalled:         make(chan struct{}),
		getCalled:         make(chan struct{}),
	}
}

// Get returns the current value.
func (ts *Setter[T, E]) Get() (s T) {
	ts.mu.Lock()
	s = ts.val
	ts.mu.Unlock()
	return
}

// Set overwrites the current value.
func (ts *Setter[T, E]) Set(s T) {
	ts.mu.Lock()
	ts.val = s
	ts.mu.Unlock()
}

// Err returns the currently configured setter error.
func (ts *Setter[T, E]) Err() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.err
}

// SetErr updates the setter error returned by JawsSet.
func (ts *Setter[T, E]) SetErr(err error) {
	ts.mu.Lock()
	ts.err = err
	ts.mu.Unlock()
}

// SetCount returns the number of JawsSet calls.
func (ts *Setter[T, E]) SetCount() (n int) {
	ts.mu.Lock()
	n = ts.setCount
	ts.mu.Unlock()
	return
}

// GetCount returns the number of JawsGet calls.
func (ts *Setter[T, E]) GetCount() (n int) {
	ts.mu.Lock()
	n = ts.getCount
	ts.mu.Unlock()
	return
}

// JawsGet returns the current value.
func (ts *Setter[T, E]) JawsGet(*E) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

// JawsSet stores val unless an error has been configured.
func (ts *Setter[T, E]) JawsSet(_ *E, val T) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = ts.errValueUnchanged
		}
		ts.val = val
	}
	return
}

// JawsGetString returns the current value for string setters.
func (ts *Setter[string, E]) JawsGetString(*E) (val string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

// JawsGetAny returns the current value for any setters.
func (ts *Setter[any, E]) JawsGetAny(*E) (val any) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

// JawsSetAny stores val for any setters.
func (ts *Setter[any, E]) JawsSetAny(_ *E, val any) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = ts.errValueUnchanged
		}
		ts.val = val
	}
	return
}

// JawsGetHTML returns the current value for HTML getters.
func (ts *Setter[T, E]) JawsGetHTML(*E) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}
