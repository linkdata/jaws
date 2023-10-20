package jaws

import (
	"reflect"
	"testing"
)

func isNil(object interface{}) bool {
	if object != nil {
		value := reflect.ValueOf(object)
		kind := value.Kind()
		if kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil() {
			return true
		}
	}
	return false
}

type testHelper struct{ *testing.T }

func (th testHelper) Equal(a, b any) {
	th.Helper()
	if !(isNil(a) || isNil(b)) {
		if !reflect.DeepEqual(a, b) {
			th.Errorf("%#v != %#v", a, b)
		}
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
