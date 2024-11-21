package jaws

import (
	"reflect"
	"sync"
	"testing"
)

type testStringer struct{}

func (s testStringer) String() string {
	return "I_Am_A_testStringer"
}

type testPtrStringer struct{}

func (s *testPtrStringer) String() string {
	return "I_Am_A_testPtrStringer"
}

func TestStringer(t *testing.T) {
	var mu sync.Mutex
	var mu2 sync.RWMutex

	txt := "text"
	stringer := Stringer(&mu, &txt)
	if s := stringer.String(); s != "text" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{&txt}) {
		t.Error(tags)
	}

	num := int(123)
	stringer = Stringer(&mu, &num)
	if s := stringer.String(); s != "123" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{&num}) {
		t.Error(tags)
	}

	flt := float64(456.123)
	stringer = Stringer(&mu, &flt, "%.0f")
	if s := stringer.String(); s != "456" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{&flt}) {
		t.Error(tags)
	}

	teststringer := testStringer{}
	stringer = Stringer(&mu, &teststringer)
	if s := stringer.String(); s != (testStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{teststringer}) {
		t.Errorf("%#v", tags)
	}

	testptrstringer := &testPtrStringer{}
	stringer = Stringer(&mu2, testptrstringer)
	if s := stringer.String(); s != (&testPtrStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{testptrstringer}) {
		t.Errorf("%#v", tags)
	}

}
