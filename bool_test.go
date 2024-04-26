package jaws

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBool_BoolSetter(t *testing.T) {
	var s Bool
	if s.String() != "false" {
		t.Fail()
	}
	s.JawsSetBool(nil, true)
	if s.JawsGetBool(nil) != true {
		t.Fail()
	}
	if s.String() != "true" {
		t.Fail()
	}
}

func TestBool_MarshalJSON(t *testing.T) {
	var s, s2 Bool
	s.Set(true)
	b, err := json.Marshal(&s)
	if err != nil {
		t.Error(err)
	} else {
		x := string(b)
		if x != "true" {
			t.Errorf("%T %q", x, x)
		}
	}
	err = json.Unmarshal(b, &s2)
	if err != nil {
		t.Error(err)
	} else {
		if s2.Value != true {
			t.Error(s2.Value)
		}
	}
}

func TestBool_MarshalText(t *testing.T) {
	tests := []struct {
		name    string
		Value   bool
		want    []byte
		wantErr bool
	}{
		{"false", false, []byte("false"), false},
		{"true", true, []byte("true"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Bool{
				Value: tt.Value,
			}
			got, err := s.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("Bool.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bool.MarshalText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBool_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		Value   bool
		str     string
		wantErr bool
	}{
		{"false", false, "false", false},
		{"true", true, "true", false},
		{"false/err", false, "foo", true},
		{"true/err", true, "foo", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Bool{
				Value: !tt.Value,
			}
			if err := s.UnmarshalText([]byte(tt.str)); (err != nil) != tt.wantErr {
				t.Errorf("Bool.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && s.Value != !tt.Value {
				t.Errorf("Bool.UnmarshalText() = %v, want %v", s.Value, !tt.Value)
			}
			if !tt.wantErr && s.Value != tt.Value {
				t.Errorf("Bool.UnmarshalText() = %v, want %v", s.Value, tt.Value)
			}
		})
	}
}
