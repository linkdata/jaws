package jaws

import (
	"encoding/json"
	"testing"
)

func TestFloat_FloatSetter(t *testing.T) {
	var s Float
	if s.String() != "0" {
		t.Fail()
	}
	s.JawsSetFloat(nil, 1)
	if s.JawsGetFloat(nil) != 1 {
		t.Fail()
	}
	if s.String() != "1" {
		t.Fail()
	}
}

func TestFloat_MarshalJSON(t *testing.T) {
	var s, s2 Float
	s.Set(1)
	b, err := json.Marshal(&s)
	if err != nil {
		t.Error(err)
	} else {
		x := string(b)
		if x != "1" {
			t.Errorf("%T %q", x, x)
		}
	}
	err = json.Unmarshal(b, &s2)
	if err != nil {
		t.Error(err)
	} else {
		if s2.Value != 1 {
			t.Error(s2.Value)
		}
	}
}
