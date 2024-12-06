package jaws

import (
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestBind_Hook_Success_panic(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Fail()
		}
	}()
	var mu sync.Mutex
	var val string
	Bind(&mu, &val).Success(func(n int) {})
	t.Fail()
}

func TestBind_Hook_Success_breaksonerr(t *testing.T) {
	var mu sync.Mutex
	var val string

	calls1 := 0
	calls2 := 0
	calls3 := 0
	bind1 := Bind(&mu, &val).
		Success(func() {
			calls1++
		}).
		Success(func() error {
			calls2++
			return io.EOF
		}).
		Success(func() {
			calls3++
		})
	if err := bind1.JawsSet(nil, "foo"); err != io.EOF {
		t.Error(err)
	}
	if calls1 != 1 {
		t.Error(calls1)
	}
	if calls2 != 1 {
		t.Error(calls2)
	}
	if calls3 != 0 {
		t.Error(calls3)
	}
}

func testBind_Hook_Success[T comparable](t *testing.T, testval T) {
	var mu sync.Mutex
	var val T
	var blankval T

	calls1 := 0
	bind1 := Bind(&mu, &val).
		Success(func() {
			calls1++
		})
	if err := bind1.JawsSet(nil, testval); err != nil {
		t.Error(err)
	}
	if x := bind1.JawsGet(nil); x != testval {
		t.Error(x)
	}
	if err := bind1.JawsSet(nil, testval); err != ErrValueUnchanged {
		t.Error(err)
	}
	if calls1 != 1 {
		t.Error(calls1)
	}
	tags1 := MustTagExpand(nil, bind1)
	if !reflect.DeepEqual(tags1, []any{&val}) {
		t.Error(tags1)
	}

	calls2 := 0
	bind2 := bind1.
		Success(func() (err error) {
			calls2++
			if calls1 <= calls2 {
				t.Error(calls1, calls2)
			}
			return
		})
	if err := bind2.JawsSet(nil, blankval); err != nil {
		t.Error(err)
	}
	if calls1 != 2 {
		t.Error(calls1)
	}
	if calls2 != 1 {
		t.Error(calls2)
	}
	tags2 := MustTagExpand(nil, bind2)
	if !reflect.DeepEqual(tags2, []any{&val}) {
		t.Error(tags2)
	}

	calls3 := 0
	bind3 := bind2.
		Success(func(*Element) {
			calls3++
			if calls2 <= calls3 {
				t.Error(calls2, calls3)
			}
		})
	if err := bind3.JawsSet(nil, testval); err != nil {
		t.Error(err)
	}
	if calls1 != 3 {
		t.Error(calls1)
	}
	if calls2 != 2 {
		t.Error(calls2)
	}
	if calls3 != 1 {
		t.Error(calls3)
	}

	calls4 := 0
	bind4 := bind3.
		Success(func(*Element) (err error) {
			calls4++
			if calls3 <= calls4 {
				t.Error(calls3, calls4)
			}
			return
		})
	if err := bind4.JawsSet(nil, blankval); err != nil {
		t.Error(err)
	}
	if calls1 != 4 {
		t.Error(calls1)
	}
	if calls2 != 3 {
		t.Error(calls2)
	}
	if calls3 != 2 {
		t.Error(calls3)
	}
	if calls4 != 1 {
		t.Error(calls4)
	}
}

func testBind_Hook_Set[T comparable](t *testing.T, testval T) {
	var mu sync.Mutex
	var val T
	var blankval T

	calls1 := 0
	bind1 := Bind(&mu, &val).
		SetLocked(func(bind Binder[T], elem *Element, value T) (err error) {
			calls1++
			return bind.JawsSetLocked(elem, value)
		})
	if err := bind1.JawsSet(nil, testval); err != nil {
		t.Error(err)
	}
	if x := bind1.JawsGet(nil); x != testval {
		t.Error(x)
	}
	if err := bind1.JawsSet(nil, testval); err != ErrValueUnchanged {
		t.Error(err)
	}
	if calls1 != 2 {
		t.Error(calls1)
	}
	tags1 := MustTagExpand(nil, bind1)
	if !reflect.DeepEqual(tags1, []any{&val}) {
		t.Error(tags1)
	}

	calls2 := 0
	bind2 := bind1.
		SetLocked(func(bind Binder[T], elem *Element, value T) (err error) {
			calls2++
			return bind.JawsSetLocked(elem, value)
		})
	if err := bind2.JawsSet(nil, blankval); err != nil {
		t.Error(err)
	}
	if calls1 != 3 {
		t.Error(calls1)
	}
	if calls2 != 1 {
		t.Error(calls2)
	}
	tags2 := MustTagExpand(nil, bind2)
	if !reflect.DeepEqual(tags2, []any{&val}) {
		t.Error(tags2)
	}
}

func testBind_Hook_Get[T comparable](t *testing.T, testval T) {
	var mu sync.Mutex
	var val T
	var blankval T

	calls1 := 0
	bind1 := Bind(&mu, &val).
		GetLocked(func(bind Binder[T], elem *Element) (value T) {
			calls1++
			return bind.JawsGetLocked(elem)
		})
	if err := bind1.JawsSet(nil, testval); err != nil {
		t.Error(err)
	}
	if x := bind1.JawsGet(nil); x != testval {
		t.Error(x)
	}
	if err := bind1.JawsSet(nil, testval); err != ErrValueUnchanged {
		t.Error(err)
	}
	if calls1 != 1 {
		t.Error(calls1)
	}
	tags1 := MustTagExpand(nil, bind1)
	if !reflect.DeepEqual(tags1, []any{&val}) {
		t.Error(tags1)
	}

	calls2 := 0
	bind2 := bind1.
		GetLocked(func(bind Binder[T], elem *Element) (value T) {
			calls2++
			return bind.JawsGetLocked(elem)
		})
	if err := bind2.JawsSet(nil, blankval); err != nil {
		t.Error(err)
	}
	if x := bind2.JawsGet(nil); x != blankval {
		t.Error(x)
	}
	if calls1 != 2 {
		t.Error(calls1)
	}
	if calls2 != 1 {
		t.Error(calls2)
	}
	tags2 := MustTagExpand(nil, bind2)
	if !reflect.DeepEqual(tags2, []any{&val}) {
		t.Error(tags2)
	}
}

func testBind_Hooks[T comparable](t *testing.T, testval T) {
	testBind_Hook_Success(t, testval)
	testBind_Hook_Set(t, testval)
	testBind_Hook_Get(t, testval)
}

func testBind_StringSetter(t *testing.T, v StringSetter) {
	val := v.JawsGetString(nil) + "!"
	if err := v.JawsSetString(nil, val); err != nil {
		t.Error(err)
	}
	if x := v.JawsGetString(nil); x != val {
		t.Error(x)
	}
}

func TestBindFunc_String(t *testing.T) {
	var mu sync.Mutex
	var val string

	testBind_Hooks(t, "foo")
	testBind_StringSetter(t, Bind(&mu, &val).(StringSetter))
	testBind_StringSetter(t, Bind(&mu, &val).Success(func() (err error) { return }).(StringSetter))
}

func testBind_FloatSetter(t *testing.T, v FloatSetter) {
	val := v.JawsGetFloat(nil) + 1
	if err := v.JawsSetFloat(nil, val); err != nil {
		t.Error(err)
	}
	if x := v.JawsGetFloat(nil); x != val {
		t.Error(x)
	}
}

func TestBindFunc_Float(t *testing.T) {
	var mu sync.Mutex
	var val float64

	testBind_Hooks(t, float64(1.23))
	testBind_FloatSetter(t, Bind(&mu, &val).(FloatSetter))
	testBind_FloatSetter(t, Bind(&mu, &val).Success(func() (err error) { return }).(FloatSetter))
}

func testBind_BoolSetter(t *testing.T, v BoolSetter) {
	val := !v.JawsGetBool(nil)
	if err := v.JawsSetBool(nil, val); err != nil {
		t.Error(err)
	}
	if x := v.JawsGetBool(nil); x != val {
		t.Error(x)
	}
}

func TestBindFunc_Bool(t *testing.T) {
	var mu sync.Mutex
	var val bool

	testBind_Hooks(t, true)
	testBind_BoolSetter(t, Bind(&mu, &val).(BoolSetter))
	testBind_BoolSetter(t, Bind(&mu, &val).Success(func() (err error) { return }).(BoolSetter))
}

func testBind_TimeSetter(t *testing.T, v TimeSetter) {
	val := v.JawsGetTime(nil).Add(time.Second)
	if err := v.JawsSetTime(nil, val); err != nil {
		t.Error(err)
	}
	if x := v.JawsGetTime(nil); x != val {
		t.Error(x)
	}
}

func TestBindFunc_Time(t *testing.T) {
	var mu sync.Mutex
	var val time.Time

	testBind_Hooks(t, time.Now())
	testBind_TimeSetter(t, Bind(&mu, &val).(TimeSetter))
	testBind_TimeSetter(t, Bind(&mu, &val).Success(func() (err error) { return }).(TimeSetter))
}
