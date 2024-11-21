package jaws

import (
	"reflect"
	"testing"
)

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
		t.Error("not equal")
	}
	if s := stringer.String(); s != (testStringer{}).String() {
		t.Error(s)
	}
	if tags := MustTagExpand(nil, stringer); !reflect.DeepEqual(tags, []any{teststringer}) {
		t.Error(tags)
	}

}
