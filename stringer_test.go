package jaws

import (
	"reflect"
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
	txt := "text"
	stringer := Stringer(&txt)
	if s := stringer.String(); s != "text" {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{&txt}) {
		t.Error(tags)
	}

	num := int(123)
	stringer = Stringer(&num)
	if s := stringer.String(); s != "123" {
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
	if s := stringer.String(); s != (testStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{teststringer}) {
		t.Error(tags)
	}

	testptrstringer := &testPtrStringer{}
	stringer = Stringer(testptrstringer)
	if !reflect.DeepEqual(stringer, testptrstringer) {
		t.Errorf("%#v != %#v", stringer, testptrstringer)
	}
	if s := stringer.String(); s != (&testPtrStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{testptrstringer}) {
		t.Error(tags)
	}

}
