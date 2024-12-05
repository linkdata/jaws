package jaws

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestBindFunc_String(t *testing.T) {
	var mu sync.Mutex
	var val string
	called1 := 0
	called2 := 0
	called3 := 0
	v := Bind(&mu, &val).SetHook(func(bind Binder[string], elem *Element, value string) (err error) {
		called1++
		return bind.JawsSetLocked(elem, value)
	})
	if err := v.JawsSetString(nil, "foo"); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetString(nil); s != "foo" {
		t.Error(s)
	}
	if called1 != 1 {
		t.Error(called1)
	}
	if tags := MustTagExpand(nil, v); !reflect.DeepEqual(tags, []any{&val}) {
		t.Error(tags)
	}
	v2 := v.SetHook(func(bind Binder[string], elem *Element, value string) (err error) {
		called2++
		return bind.JawsSetLocked(elem, value)
	}).Success(func() {
		called3++
	})
	if err := v2.JawsSetString(nil, "bar"); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetString(nil); s != "bar" {
		t.Error(s)
	}
	v3 := v2.GetHook(func(bind Binder[string], elem *Element) (value string) {
		return "quux"
	})
	if s := v3.JawsGetString(nil); s != "quux" {
		t.Error(s)
	}
	if called1 != 2 {
		t.Error(called1)
	}
	if called2 != 1 {
		t.Error(called2)
	}
	if called3 != 1 {
		t.Error(called3)
	}
}

func TestBindFunc_Float(t *testing.T) {
	var mu sync.Mutex
	var val float64
	called := 0
	v := Bind(&mu, &val).SetHook(func(bind Binder[float64], elem *Element, value float64) (err error) {
		called++
		return bind.JawsSetLocked(elem, value)
	})
	if err := v.JawsSetFloat(nil, 123); err != nil {
		t.Error(err)
	}
	if x := v.JawsGetFloat(nil); x != 123 {
		t.Error(x)
	}
	called2 := 0
	called3 := 0
	v2 := v.SetHook(func(bind Binder[float64], elem *Element, value float64) (err error) {
		called2++
		return bind.JawsSetLocked(elem, value)
	}).Success(func() {
		called3++
	})
	if err := v2.JawsSetFloat(nil, 345); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetFloat(nil); s != 345 {
		t.Error(s)
	}
	v3 := v2.GetHook(func(bind Binder[float64], elem *Element) (value float64) {
		return 234
	})
	if called2 != 2 {
		t.Error(called)
	}
	if called3 != 1 {
		t.Error(called)
	}
	if s := v3.JawsGetFloat(nil); s != 234 {
		t.Error(s)
	}
	if called != 1 {
		t.Error(called)
	}
}

func TestBindFunc_Bool(t *testing.T) {
	var mu sync.Mutex
	var val bool
	called := 0
	v := Bind(&mu, &val).SetHook(func(bind Binder[bool], elem *Element, value bool) (err error) {
		called++
		return bind.JawsSetLocked(elem, value)
	})
	if err := v.JawsSetBool(nil, true); err != nil {
		t.Error(err)
	}
	if x := v.JawsGetBool(nil); x != true {
		t.Error(x)
	}
	called2 := 0
	called3 := 0
	v2 := v.SetHook(func(bind Binder[bool], elem *Element, value bool) (err error) {
		called2++
		return bind.JawsSetLocked(elem, value)
	}).Success(func() {
		called3++
	})
	if err := v2.JawsSetBool(nil, false); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetBool(nil); s != false {
		t.Error(s)
	}
	v3 := v2.GetHook(func(bind Binder[bool], elem *Element) (value bool) {
		return true
	})
	if called2 != 2 {
		t.Error(called)
	}
	if called3 != 1 {
		t.Error(called)
	}
	if s := v3.JawsGetBool(nil); !s {
		t.Error(s)
	}
	if called != 1 {
		t.Error(called)
	}
}

func TestBindFunc_Time(t *testing.T) {
	var mu sync.Mutex
	var val time.Time
	called := 0
	v := Bind(&mu, &val).SetHook(func(bind Binder[time.Time], elem *Element, value time.Time) (err error) {
		called++
		return bind.JawsSetLocked(elem, value)
	})
	want := time.Now()
	if err := v.JawsSetTime(nil, want); err != nil {
		t.Error(err)
	}
	if x := v.JawsGetTime(nil); x != want {
		t.Error(x)
	}
	called2 := 0
	called3 := 0
	v2 := v.SetHook(func(bind Binder[time.Time], elem *Element, value time.Time) (err error) {
		called2++
		return bind.JawsSetLocked(elem, value)
	}).Success(func() {
		called3++
	})
	want = want.Add(time.Second)
	if err := v2.JawsSetTime(nil, want); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetTime(nil); s != want {
		t.Error(s)
	}
	v3 := v2.GetHook(func(bind Binder[time.Time], elem *Element) (value time.Time) {
		return time.Time{}
	})
	if called2 != 2 {
		t.Error(called)
	}
	if called3 != 1 {
		t.Error(called)
	}
	if s := v3.JawsGetTime(nil); !s.IsZero() {
		t.Error(s)
	}
	if called != 1 {
		t.Error(called)
	}
}
