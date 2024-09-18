package jaws

import (
	"sync"
	"testing"
	"time"
)

func Test_Bind_string(t *testing.T) {
	var mu sync.Mutex
	var val string
	v := Bind(&mu, &val)
	if err := v.JawsSetString(nil, "foo"); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetString(nil); s != "foo" {
		t.Error(s)
	}
}

func Test_Bind_float64(t *testing.T) {
	var mu sync.Mutex
	var val float64
	v := Bind(&mu, &val)
	if err := v.JawsSetFloat(nil, 1.2); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetFloat(nil); s != 1.2 {
		t.Error(s)
	}
}

func Test_Bind_bool(t *testing.T) {
	var mu sync.Mutex
	var val bool
	v := Bind(&mu, &val)
	if err := v.JawsSetBool(nil, true); err != nil {
		t.Error(err)
	}
	if s := v.JawsGetBool(nil); s != true {
		t.Error(s)
	}
}

func Test_Bind_Time(t *testing.T) {
	var mu sync.Mutex
	var val time.Time
	v := Bind(&mu, &val)
	now := time.Now()

	if err := v.JawsSetTime(nil, now); err != nil {
		t.Error(err)
	}
	if v := v.JawsGetTime(nil); v != now {
		t.Error(v)
	}
}
