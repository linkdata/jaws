package jaws

import (
	"reflect"
	"testing"
)

type testHelper struct{ *testing.T }

func (th testHelper) equal(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	aIsNil, aType := testNil(a)
	bIsNil, bType := testNil(b)
	if !(aIsNil && bIsNil) {
		return false
	}
	return aType == nil || bType == nil || (aType == bType)
}

func (th testHelper) Equal(a, b any) {
	th.Helper()
	if !th.equal(a, b) {
		th.Errorf("%#v != %#v", a, b)
	}
}

func (th testHelper) True(a bool) {
	th.Helper()
	if !a {
		th.Error("not true")
	}
}

func (th testHelper) NoErr(err error) {
	th.Helper()
	if err != nil {
		th.Error(err)
	}
}

func testNil(object any) (bool, reflect.Type) {
	if object == nil {
		return true, nil
	}
	value := reflect.ValueOf(object)
	kind := value.Kind()
	return kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil(), value.Type()
}

func Test_testHelper(t *testing.T) {
	is := testHelper{t}

	mustEqual := func(a, b any) {
		t.Helper()
		if !is.equal(a, b) {
			t.Errorf("%#v != %#v", a, b)
		}
	}

	mustNotEqual := func(a, b any) {
		t.Helper()
		if is.equal(a, b) {
			t.Errorf("%#v == %#v", a, b)
		}
	}

	mustEqual(1, 1)
	mustEqual(nil, nil)
	mustEqual(nil, (*testHelper)(nil))
	mustNotEqual(1, nil)
	mustNotEqual(nil, 1)
	mustNotEqual((*testing.T)(nil), 1)
	mustNotEqual(1, 2)
	mustNotEqual((*testing.T)(nil), (*testHelper)(nil))
	mustNotEqual(int(1), int32(1))
}
