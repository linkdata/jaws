package jaws

import (
	"sync"
	"testing"
	"time"
)

func TestAfterSet_String(t *testing.T) {
	var mu sync.Mutex
	var value string
	var called int

	as := AfterSet(Bind(&mu, &value), func(bind Binding[string]) (err error) {
		called++
		return
	})
	if err := as.JawsSetString(nil, "foo"); err != nil {
		t.Error(err)
	}
	if called != 1 {
		t.Error(called)
	}
	if tag := as.JawsGetTag(nil); tag != &value {
		t.Error(tag)
	}
	if err := as.JawsSetString(nil, "foo"); err != ErrValueUnchanged {
		t.Error(err)
	}
	if called != 1 {
		t.Error(called)
	}
}

func TestAfterSet_Float(t *testing.T) {
	var mu sync.Mutex
	var value float64
	var called bool

	as := AfterSet(Bind(&mu, &value), func(bind Binding[float64]) (err error) {
		called = true
		return
	})
	if err := as.JawsSetFloat(nil, 1); err != nil {
		t.Error(err)
	}
	if !called {
		t.Error(called)
	}
}

func TestAfterSet_Bool(t *testing.T) {
	var mu sync.Mutex
	var value bool
	var called bool

	as := AfterSet(Bind(&mu, &value), func(bind Binding[bool]) (err error) {
		called = true
		return
	})
	if err := as.JawsSetBool(nil, !value); err != nil {
		t.Error(err)
	}
	if !called {
		t.Error(called)
	}
}

func TestAfterSet_Time(t *testing.T) {
	var mu sync.Mutex
	var value time.Time
	var called bool

	as := AfterSet(Bind(&mu, &value), func(bind Binding[time.Time]) (err error) {
		called = true
		return
	})
	if err := as.JawsSetTime(nil, time.Now()); err != nil {
		t.Error(err)
	}
	if !called {
		t.Error(called)
	}
}
