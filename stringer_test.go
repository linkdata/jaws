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
	stringer := Stringer(&txt)
	if s := Bind(&mu, &stringer).JawsGetString(nil); s != "text" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{&txt}) {
		t.Error(tags)
	}

	num := int(123)
	stringer = Stringer(&num)
	if s := Bind(&mu, &stringer).JawsGetString(nil); s != "123" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{&num}) {
		t.Error(tags)
	}

	teststringer := testStringer{}
	stringer = Stringer(&teststringer)
	if !reflect.DeepEqual(stringer, teststringer) {
		t.Errorf("%#v != %#v", stringer, teststringer)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{teststringer}) {
		t.Errorf("%#v", tags)
	}
	b1 := Bind(&mu, &stringer)
	if s := b1.JawsGetString(nil); s != (testStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, b1); !reflect.DeepEqual(tags, []any{teststringer}) {
		t.Errorf("%#v", tags)
	}
	if err := b1.JawsSetString(nil, "x"); err != ErrValueNotSettable {
		t.Error(err)
	}

	testptrstringer := &testPtrStringer{}
	stringer = Stringer(testptrstringer)
	if !reflect.DeepEqual(stringer, testptrstringer) {
		t.Errorf("%#v != %#v", stringer, testptrstringer)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{testptrstringer}) {
		t.Errorf("%#v", tags)
	}

	b2 := Bind(&mu2, &stringer)
	if s := b2.JawsGetString(nil); s != (&testPtrStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, b2); !reflect.DeepEqual(tags, []any{testptrstringer}) {
		t.Errorf("%#v", tags)
	}
	if err := b2.JawsSetString(nil, "x"); err != ErrValueNotSettable {
		t.Error(err)
	}

	b3 := Bind(&mu2, testptrstringer)
	if s := b3.JawsGetString(nil); s != (&testPtrStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, b3); !reflect.DeepEqual(tags, []any{testptrstringer}) {
		t.Errorf("%#v", tags)
	}
	if err := b3.JawsSetString(nil, "x"); err != ErrValueNotSettable {
		t.Error(err)
	}
}
